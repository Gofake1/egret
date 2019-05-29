package main

import (
	"container/heap" // Pop, Push
	"log"            // Println
	"strconv"        // FormatUint

	"github.com/emersion/go-imap"        // MailboxInfo, Message, SeqSet
	"github.com/emersion/go-imap/client" // Client
)

func fetchMboxes(c *client.Client) []string {
	defer log.Println("Fetched mailbox infos")
	ch := make(chan *imap.MailboxInfo)
	go func() {
		err := c.List("", "*", ch)
		if err != nil {
			log.Fatal(err)
		}
	}()
	mboxes := []string{}
	for info := range ch {
		mboxes = append(mboxes, info.Name)
	}
	return mboxes
}

func fetchMessages(c *client.Client, mboxName string) []*imap.Message {
	defer log.Println("Fetched " + mboxName)
	ms, err := c.Select(mboxName, false)
	if err != nil {
		log.Fatal(err)
	}

	if ms.Messages < 1 {
		return []*imap.Message{}
	}

	ch := make(chan *imap.Message)
	go func() {
		s := new(imap.SeqSet)
		s.AddRange(uint32(1), ms.Messages)
		// `imap.FetchBody` ("BODY") doesn't work, must be "BODY[]"
		items := []imap.FetchItem{imap.FetchItem("BODY[]"), imap.FetchEnvelope, imap.FetchUid}
		err := c.Fetch(s, items, ch)
		if err != nil {
			log.Fatal(err)
		}
	}()
	h := newMessageHeap()
	for msg := range ch {
		heap.Push(h, msg)
	}
	msgs := []*imap.Message{}
	for h.Len() > 0 {
		msgs = append(msgs, heap.Pop(h).(*imap.Message))
	}
	return msgs
}

func fetchMessage(c *client.Client, mboxName string, uid uint32) *imap.Message {
	defer log.Println("Fetched " + mboxName + " " + strconv.FormatUint(uint64(uid), 10))
	_, err := c.Select(mboxName, false)
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan *imap.Message)
	go func() {
		s := new(imap.SeqSet)
		s.AddNum(uid)
		items := []imap.FetchItem{imap.FetchItem("BODY[]"), imap.FetchEnvelope}
		err := c.UidFetch(s, items, ch)
		if err != nil {
			log.Fatal(err)
		}
	}()
	return <-ch
}
