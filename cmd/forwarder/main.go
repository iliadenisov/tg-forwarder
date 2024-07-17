package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram"
	"github.com/iliadenisov/tg-client/internal/hasher"
	"github.com/iliadenisov/tg-client/internal/registry"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	cfg := zap.NewDevelopmentConfig()
	if os.Getenv("APP_ENV") != "development" {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	zap.ReplaceGlobals(zap.Must(cfg.Build()))
}

func run(ctx context.Context) error {
	var noUpdates bool
	hasher := hasher.NewHasher()

	fwd, err := registry.GetForwardMap("FORWARD_MAP")
	if err != nil {
		zap.L().Sugar().Infof("forwarding configuration cannot be read: %s", err)
		noUpdates = true
	}

	registry := registry.NewRegistry(ctx)

	clientLog := zap.Must(zap.NewDevelopment(zap.IncreaseLevel(zapcore.WarnLevel)))
	defer func() { _ = clientLog.Sync() }()

	d := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler:      d,
		AccessHasher: hasher,
		Logger:       clientLog.Named("gaps"),
	})

	codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		fmt.Print("Enter code: ")
		code, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(code), nil
	}

	flow := auth.NewFlow(
		auth.Env("ACCOUNT_", auth.CodeAuthenticatorFunc(codePrompt)),
		auth.SendCodeOptions{})

	dc := telegram.DeviceConfig{
		DeviceModel:   "unix",
		SystemVersion: "Ubuntu 22.04",
		AppVersion:    "Go Fw Cli 1.0.0",
	}
	dc.SetDefaults()
	client, err := telegram.ClientFromEnvironment(telegram.Options{
		Logger:        clientLog,
		UpdateHandler: gaps,
		Device:        dc,
		NoUpdates:     noUpdates,
	})
	if err != nil {
		return err
	}

	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		message, ok := update.GetMessage().(*tg.Message)
		if !ok {
			return nil
		}

		channel, ok := message.GetPeerID().(*tg.PeerChannel)
		if !ok {
			return nil
		}

		if _, ok := fwd[channel.ChannelID]; !ok {
			return nil
		}

		registry.RegisterMessage(channel.ChannelID, message.GroupedID, message.GetID(), message.Date)
		return nil
	})

	registry.OnMessageForward(func(srcChan int64, msgId []int) {
		dstChan, ok := fwd[srcChan]
		if !ok {
			zap.L().Sugar().Errorf("source channel_id=%d forward rejected (no destination channel configured)", srcChan)
			return
		}
		zap.L().Sugar().Debugf("source channel_id=%d forwarding %d message(s) to channel_id=%d", srcChan, len(msgId), dstChan)
		srcAccessHash, ok, _ := hasher.GetChannelAccessHash(ctx, 0, srcChan)
		if !ok {
			zap.L().Sugar().Errorf("source channel_id=%d access hash unknown", srcChan)
			return
		}
		dstAccessHash, ok, _ := hasher.GetChannelAccessHash(ctx, 0, dstChan)
		if !ok {
			zap.L().Sugar().Errorf("destination channel_id=%d access hash unknown", dstChan)
			return
		}
		rndIds := make([]int64, len(msgId))
		for i := range msgId {
			rndIds[i] = int64(rand.Int())
		}
		_, err := client.API().MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
			FromPeer:   &tg.InputPeerChannel{ChannelID: srcChan, AccessHash: srcAccessHash},
			ToPeer:     &tg.InputPeerChannel{ChannelID: dstChan, AccessHash: dstAccessHash},
			ID:         msgId,
			RandomID:   rndIds,
			DropAuthor: true,
			Silent:     false,
			Background: true,
		})
		if err != nil {
			zap.L().Sugar().Errorln("forwardMessages error:", err)
		}
	})

	return client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return errors.Wrap(err, "auth")
		}

		user, err := client.Self(ctx)
		if err != nil {
			return errors.Wrap(err, "call self")
		}

		mdc, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerChat{},
		})
		if err != nil {
			return errors.Wrap(err, "call MessagesGetDialogs")
		}

		if am, ok := mdc.AsModified(); ok {
			for _, chat := range am.GetChats() {
				if channel, ok := asChannel(chat); ok {
					if noUpdates {
						zap.L().Sugar().Info("known channel: username=", channel.Username, ", id=", channel.ID, ", title=", channel.Title)
					}
					hasher.SetChannelAccessHash(ctx, 0, channel.ID, channel.AccessHash)
				}
			}
		}

		if noUpdates {
			return nil
		}

		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				zap.L().Info("client started")
			},
		})
	})
}

func asChannel(chat tg.ChatClass) (*tg.Channel, bool) {
	if nfc, ok := chat.AsNotForbidden(); ok {
		if ch, ok := nfc.(*tg.Channel); ok && ch.TypeID() == tg.ChannelTypeID {
			return ch, true
		}
	}
	return nil, false
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
