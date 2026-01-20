package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	cutWords = []string{
		"алиса",
		"надо",
		"купить",
		"запусти",
		"навык",
		"список",
		"покупок",
	}

	tgHelper *TgHelper
)

const (
	tgTimeout = 5 * time.Second
)

// ================= MAIN =================
func main() {
	var err error
	tgHelper, err = NewTgHelper()
	if err != nil {
		log.Fatalf("failed to initialize tg helper: %v", err)
	}

	r := gin.Default()
	r.POST("/", postHandler)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("Server started on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// ================= JSON =================

type AliceRequest struct {
	Session struct {
		SessionID string `json:"session_id"`
	} `json:"session"`

	Version string `json:"version"`

	Request struct {
		OriginalUtterance string `json:"original_utterance"`
	} `json:"request"`
}

type AliceResponse struct {
	Session  string `json:"session"`
	Version  string `json:"version"`
	Response struct {
		Text       string `json:"text,omitempty"`
		TTS        string `json:"tts,omitempty"`
		EndSession bool   `json:"end_session"`
	} `json:"response"`
}

// ================= HANDLER =================

func postHandler(c *gin.Context) {
	var req AliceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := AliceResponse{
		Session: req.Session.SessionID,
		Version: req.Version,
	}
	resp.Response.EndSession = false

	handleDialog(&resp, req.Request.OriginalUtterance)

	c.JSON(http.StatusOK, resp)
}

// ================= LOGIC =================

func handleDialog(res *AliceResponse, utterance string) {
	utterance = normalizeUtterance(utterance)

	if utterance == "" {
		res.Response.Text = "Добавьте товары в список покупок"
		return
	}

	log.Printf("final utterance: %q", utterance)

	go func(msg string) {
		ctx, cancel := context.WithTimeout(context.Background(), tgTimeout)
		defer cancel()

		if tgHelper.needToEditMessage() {
			if err := tgHelper.editTelegramMessage(ctx, msg); err != nil {
				log.Printf("telegram edit error: %v", err)
			}
			return
		}

		if err := tgHelper.sendTelegramMessage(ctx, msg); err != nil {
			log.Printf("telegram send error: %v", err)
		}
	}(utterance)

	res.Response.Text = "Добавила в список покупок"
}

// ================= HELPERS =================

func normalizeUtterance(u string) string {
	u = strings.ToLower(u)

	for _, w := range cutWords {
		u = strings.ReplaceAll(u, w, "")
	}

	u = strings.TrimSpace(u)
	u = strings.Trim(u, ",.!?")

	return u
}
