package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const editThreshold = 30 * time.Minute

type TgHelper struct {
	Bot    *tgbotapi.BotAPI
	ChatID int64

	mu          sync.Mutex
	LastMessage *LastMessage
}

type LastMessage struct {
	MessageID int
	SentAt    time.Time
	Text      string
}

func NewTgHelper() (*TgHelper, error) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		return nil, err
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		return nil, err
	}

	return &TgHelper{
		Bot:    bot,
		ChatID: chatID,
	}, nil
}

func (t *TgHelper) sendTelegramMessage(ctx context.Context, text string) error {
	msg := tgbotapi.NewMessage(t.ChatID, text)

	ch := make(chan error, 1)
	var msgID int

	go func() {
		resp, err := t.Bot.Send(msg)
		if err == nil {
			msgID = resp.MessageID
		}
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err == nil {
			t.mu.Lock()
			t.LastMessage = &LastMessage{
				MessageID: msgID,
				SentAt:    time.Now(),
				Text:      text,
			}
			t.mu.Unlock()
		}
		return err
	}
}

func (t *TgHelper) editTelegramMessage(ctx context.Context, newMsg string) error {
	t.mu.Lock()
	if t.LastMessage == nil {
		t.mu.Unlock()
		return fmt.Errorf("no message to edit")
	}

	messageID := t.LastMessage.MessageID
	prevText := t.LastMessage.Text
	t.mu.Unlock()

	text := lastMessageText(prevText, newMsg)

	edit := tgbotapi.NewEditMessageText(t.ChatID, messageID, text)

	ch := make(chan error, 1)
	go func() {
		_, err := t.Bot.Send(edit)
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err == nil {
			t.mu.Lock()
			t.LastMessage.SentAt = time.Now()
			t.LastMessage.Text = text
			t.mu.Unlock()
		}
		return err
	}
}

func (t *TgHelper) needToEditMessage() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.LastMessage != nil &&
		time.Since(t.LastMessage.SentAt) < editThreshold
}

func lastMessageText(prevMsg, newMsg string) string {
	return fmt.Sprintf("%s\n%s", prevMsg, newMsg)
}
