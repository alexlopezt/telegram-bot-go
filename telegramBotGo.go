package telegrambotgo

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

const (
	lockKey              string        = "bot_redis_lock"
	lockExpiration       time.Duration = 30 * time.Second
	anonymous_group_user int64         = 1087968824
)

var telegramBot *bot.Bot
var telegramBotContext, redisContext context.Context
var cancel context.CancelFunc
var regularExpressionTgLin *regexp.Regexp

func TelegramBot(redisClient *redis.Client) {
	telegramBot = nil
	lockID := uuid.NewString()

	redisContext = context.Background()

	acquired, err := redisClient.SetNX(redisContext, lockKey, lockID, lockExpiration).Result()
	if err != nil {
		fmt.Printf("Failed trying to acquire telegram bot lock: %+v", err)
		return
	}

	if !acquired {
		fmt.Println("Other instance is already listening telegram bot")
		time.Sleep(lockExpiration / 2)
		go TelegramBot(redisClient)
		return
	}

	fmt.Println("Telegram bot lock acquired. Listening bot...")

	ticker := time.NewTicker(lockExpiration / 2)
	defer ticker.Stop()

	done := make(chan bool)

	go keepLockAlive(redisClient, lockID, ticker, done)

	startTelegramBot()

	redisClient.Del(redisContext, lockKey)
	done <- true
}

func UnlockFromCache(redisClient *redis.Client) {
	redisClient.Del(redisContext, lockKey)
}

func keepLockAlive(redisClient *redis.Client, lockID string, ticker *time.Ticker, done chan bool) {
	for {
		select {
		case <-ticker.C:
			// Renueva el bloqueo sólo si aún es el dueño
			result, err := redisClient.Eval(redisContext, `
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("expire", KEYS[1], ARGV[2])
			else
				return 0
			end
		`, []string{lockKey}, lockID, int(lockExpiration.Seconds())).Result()
			if err != nil || result == int64(0) {
				fmt.Printf("failed renewing telegram bot lock. Perhaps other instance is getting it")
				cancel()
				time.Sleep(lockExpiration / 2)
				go TelegramBot(redisClient)
				done <- true
			} else {
				fmt.Println("Telegram bot lock renewed")

			}
		case <-done:
			ticker.Stop()
			return
		}
	}
}
func startTelegramBot() {
	var err error
	telegramBotContext, cancel = signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
		bot.WithErrorsHandler(fallBack),
	}

	telegramBot, err = bot.New(os.Getenv("BOT_TOKEN"), opts...)
	if nil != err {

		panic(err)
	}

	telegramBot.Start(telegramBotContext)
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {

	if update.Message == nil {
		return
	}

	fmt.Printf("Message posted by user{%d} in group {%s}: %v", update.Message.From.ID, update.Message.Chat.Title, update.Message.Text)

}

func fallBack(err error) {
	fmt.Printf("%+v", err)
}

func checkTelegramLink(text string) bool {
	if regularExpressionTgLin == nil {
		regularExpressionTgLin = regexp.MustCompile(`(?i)https?://t\.me/`)
	}

	return regularExpressionTgLin.MatchString(text)

}
