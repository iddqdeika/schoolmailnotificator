package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"schoolmailnotificator/pkg/config"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	lastMessageBoltKey             = "last_message_uid"
	uploadDirName                  = "upload"
	telegramMessageSendingInterval = time.Millisecond * 3000
)

//TODO: decompose this shit
func main() {
	os.Mkdir(uploadDirName, os.ModePerm)

	cfg, err := config.NewJsonCfg("cfg.json")
	if err != nil {
		panic(err)
	}
	chanID, err := cfg.GetInt("channel_id")
	if err != nil {
		panic(err)
	}

	token, err := cfg.GetString("telegram_token")
	if err != nil {
		panic(err)
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(true)
	if err != nil {
		panic(err)
	}
	b, err := tx.CreateBucketIfNotExists([]byte("data"))
	if err != nil {
		panic(err)
	}
	suid := b.Get([]byte(lastMessageBoltKey))
	tx.Commit()
	var uid uint32 = 1
	if len(suid) > 0 {
		i, err := strconv.Atoi(string(suid))
		if err != nil {
			panic("cant parse last message " + err.Error())
		}
		uid = uint32(i)
	}

	addr, err := cfg.GetString("imap_host")
	if err != nil {
		panic(err)
	}
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		panic(err)
	}
	username, err := cfg.GetString("username")
	if err != nil {
		panic(err)
	}
	password, err := cfg.GetString("password")
	if err != nil {
		panic(err)
	}
	err = c.Login(username, password)
	if err != nil {
		panic(err)
	}
	defer c.Logout()

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Flags for INBOX:", mbox.Flags)

	from := uid + 2
	to := uint32(0)
	if mbox.Messages > 100 && from == 1 {
		// We're using unsigned integers here, only substract if the result is > 0
		from = mbox.Messages - 100
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, "BODY[]", imap.FetchFlags, imap.FetchUid}, messages)
	}()

	res := make(chan *Message, 10)
	go func() {
		defer close(res)
		for msg := range messages {
			if msg.Uid <= uid {
				continue
			}
			processImapMessage(msg, false, res)
			err := db.Update(func(tx *bolt.Tx) error {
				b, err := tx.CreateBucketIfNotExists([]byte("data"))
				if err != nil {
					return err
				}
				err = b.Put([]byte(lastMessageBoltKey), []byte(strconv.Itoa(int(msg.Uid))))
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				panic("cant store last message: " + err.Error())
			}
		}
	}()

	for r := range res {
		h := fmt.Sprintf("новое письмо от %v\r\nна тему %v", r.From, r.Subj)
		_, err := send(bot, tgbotapi.NewMessageToChannel(strconv.Itoa(chanID), h))
		if err != nil {
			fmt.Println("cant send msg: " + err.Error())
			continue
		}
		if len(r.Text) > 0 {
			t := fmt.Sprintf("текст письма: \r\n%v", r.Text)
			_, err = send(bot, tgbotapi.NewMessageToChannel(strconv.Itoa(chanID), t))
			if err != nil {
				fmt.Println("cant send msg: " + err.Error())
				continue
			}
		}

		for _, name := range r.FileNames {
			data, err := ioutil.ReadFile(name)
			if err != nil {
				fmt.Println("cant read file: " + err.Error())
				continue
			}
			_, err = send(bot, tgbotapi.NewDocumentUpload(int64(chanID), tgbotapi.FileBytes{
				Name:  name,
				Bytes: data,
			}))
			if err != nil {
				fmt.Println("cant send file msg: " + err.Error())
				continue
			}
			os.Remove(name)
		}

		f := fmt.Sprintf("конец письма")
		_, err = send(bot, tgbotapi.NewMessageToChannel(strconv.Itoa(chanID), f))
		if err != nil {
			fmt.Println("cant send msg: " + err.Error())
			continue
		}
	}

	if err := <-done; err != nil {
		panic(err)
	}
}

func send(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	time.Sleep(telegramMessageSendingInterval)
	return bot.Send(chattable)
}

func processImapMessage(msg *imap.Message, onlyUnseen bool, resChan chan *Message) bool {
	if onlyUnseen {
		for _, f := range msg.Flags {
			if f == "\\Seen" {
				return false
			}
		}
	}
	for _, literal := range msg.Body {

		parsedMessage, err := mail.ReadMessage(literal)
		if err != nil {
			fmt.Println("cant read message: " + err.Error())
			continue
		}
		dec := new(mime.WordDecoder)
		from, err := dec.DecodeHeader(parsedMessage.Header.Get("From"))
		if err != nil {
			fmt.Println("cant parse filename: " + err.Error())
			continue
		}
		subj, err := dec.DecodeHeader(parsedMessage.Header.Get("Subject"))
		if err != nil {
			fmt.Println("cant parse filename: " + err.Error())
			continue
		}
		ct, params, err := mime.ParseMediaType(parsedMessage.Header.Get("Content-Type"))
		switch ct {
		case "multipart/alternative", "multipart/mixed":
			text, fnames := handleMultipart(parsedMessage.Body, params["boundary"], "", nil)
			resChan <- &Message{
				From:      from,
				Subj:      subj,
				Text:      text,
				FileNames: fnames,
			}
		case "text/plain":
			rm, err := mail.ReadMessage(parsedMessage.Body)
			if err != nil {
				fmt.Println("cant parse mail message: " + err.Error())
				continue
			}
			data, _ := ioutil.ReadAll(rm.Body)
			resChan <- &Message{
				From:      from,
				Subj:      subj,
				Text:      string(data),
				FileNames: nil,
			}
		default:
			return true
		}
	}
	return true
}

func handleMultipart(reader io.Reader, boundary string, text string, fnames []string) (string, []string) {
	r := multipart.NewReader(reader, boundary)
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("cant get part: " + err.Error())
			continue
		}
		media, params, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		switch media {
		case "text/plain":
			text = appendText(p, text)
			continue
		case "multipart/related", "multipart/alternative":
			text, fnames = handleMultipart(p, params["boundary"], text, fnames)
		default:
			fnames = appendFile(p, fnames)
		}
	}
	return text, fnames
}

func appendText(p *multipart.Part, text string) string {
	var r io.Reader = p
	if p.Header.Get("Content-Transfer-Encoding") == "base64" {
		r = base64.NewDecoder(base64.StdEncoding, p)
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Println("cant get part: " + err.Error())
		return text
	}
	text += string(data) + "\r\n"
	return text
}

func appendFile(p *multipart.Part, fnames []string) []string {
	dec := new(mime.WordDecoder)
	fname, err := dec.DecodeHeader(p.FileName())
	if err != nil {
		fmt.Println("cant parse filename: " + err.Error())
		return fnames
	}
	if len(fname) > 0 {
		prefix := strconv.Itoa(time.Now().Day()) +
			strconv.Itoa(int(rand.Int31n(99)))
		fname = uploadDirName + "/" + prefix + fname
		var r io.Reader = p
		if p.Header.Get("Content-Transfer-Encoding") == "base64" {
			r = base64.NewDecoder(base64.StdEncoding, p)
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			fmt.Println("cant get part: " + err.Error())
			return fnames
		}
		err = ioutil.WriteFile(fname, data, os.ModeAppend)
		if err != nil {
			fmt.Println("cant get part: " + err.Error())
			return fnames
		}
		fnames = append(fnames, fname)
	}
	return fnames
}

type Message struct {
	From      string
	Subj      string
	Text      string
	FileNames []string
}
