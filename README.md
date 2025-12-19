### yt-gateway

Instant download of YouTube videos in small 480p file size, for example, to quickly share via Telegram and save on traffic.

CLI: https://github.com/imbecility/yt-gateway/tree/main/cmd/yt-gateway
CLI-Release: https://github.com/imbecility/yt-gateway/releases/latest

Examle and simply Lib-integration:

```go
package main

import (
	"log"
	"os"

	// 1. Import Telegram API
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	// 2. Import yt-gateway library
	"github.com/imbecility/yt-gateway/pkg/gateway"
	"github.com/imbecility/yt-gateway/pkg/logger"
	// "github.com/imbecility/yt-gateway/pkg/utils" // <- for utils.ExtractVideoID("string")
)

func main() {
	// --- Step 1: Initialize Global Logger ---
	// Enable debug mode to see internal library logs in the console
	logger.SetupGlobal(true, false)

	// --- Step 2: Initialize Telegram Bot ---
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// --- Step 3: Initialize YT-Gateway Library ---
	// This single call sets up the HTTP client (TLS), all providers,
	// the downloader, and the FFmpeg wrapper.
	gw, err := gateway.New(gateway.Config{
		OutputDir:    "./temp_videos", // Directory to store downloaded files
		FFmpegPath:   "ffmpeg",        // Path to ffmpeg binary
		TimeoutSec:   60,              // Max time to find a link
		Debug:        true,            // Enable library debug logs
		ShowProgress: false,           // Disable console progress bar (not needed for bots)
	})
	if err != nil {
		log.Fatal("Failed to init gateway:", err)
	}

	// --- Step 4: Start Bot Update Loop ---
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		// Ignore non-message updates
		if update.Message == nil {
			continue
		}

		msg := update.Message
		chatID := msg.Chat.ID
		userText := msg.Text

		// Simple check: is it a YouTube link?
		// (The library handles ID extraction, but we can do a quick check here)
    // Use utils.ExtractVideoID("string")
		if userText == "" {
			continue
		}

		// 1. Notify user that processing has started
		reply := tgbotapi.NewMessage(chatID, "⏳ <b>Processing...</b>\n<i>Finding the best stream and downloading.</i>")
		reply.ParseMode = "HTML"
		statusMsg, _ := bot.Send(reply)

		// 2. THE MAGIC: Delegate everything to your library
		// gw.ProcessVideo does the following:
		// - Extracts Video ID
		// - Races all providers (yt1s, clipto, etc.) to find a working link
		// - Downloads video and audio streams
		// - Merges them via FFmpeg if necessary
		// - Returns the local file path
		videoInfo, filePath, err := gw.ProcessVideo(userText)

		// 3. Handle Errors
		if err != nil {
			// Update status message with error
			edit := tgbotapi.NewEditMessageText(chatID, statusMsg.MessageID, "❌ <b>Error:</b> "+err.Error())
			edit.ParseMode = "HTML"
			bot.Send(edit)
			continue
		}

		// 4. Send the Video
		// Note: Telegram Bot API has a 50MB limit for direct uploads.
		// For larger files, you might need a local Telegram Bot API Server.
		video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(filePath))
		video.Caption = "🎬 <b>" + videoInfo.Title + "</b>"
		video.ParseMode = "HTML"
		video.SupportsStreaming = true

		_, err = bot.Send(video)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Failed to upload video (maybe too large?)"))
		} else {
			// Delete "Processing" message on success
			bot.Send(tgbotapi.NewDeleteMessage(chatID, statusMsg.MessageID))
		}

		// 5. Cleanup
		// Remove the file from the local disk to save space
		_ = os.Remove(filePath)
	}
}
```

nano ffmpeg bins ~2 Mb:

- [for Linux](https://github.com/imbecility/yt-gateway/raw/refs/heads/main/ffmpeg_nano/ffmpeg_nano)
- [for Windows](https://github.com/imbecility/yt-gateway/raw/refs/heads/main/ffmpeg_nano/ffmpeg_nano.exe)
