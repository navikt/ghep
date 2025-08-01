package ghep

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/navikt/ghep/internal/github"
	"github.com/navikt/ghep/internal/slack"
	"github.com/navikt/ghep/internal/sql/gensql"
	"github.com/redis/go-redis/v9"
)

func MigrateRedis(ctx context.Context, log *slog.Logger, db *gensql.Queries, teams map[string]github.Team) {
	log.Info("Starting Redis migration")

	org := os.Getenv("GITHUB_ORG")

	opt, err := redis.ParseURL(os.Getenv("REDIS_URI_EVENTS"))
	if err != nil {
		log.Error("creating Redis URL", "err", err.Error())
		return
	}

	opt.Username = os.Getenv("REDIS_USERNAME_EVENTS")
	opt.Password = os.Getenv("REDIS_PASSWORD_EVENTS")
	rdb := redis.NewClient(opt)

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Error("connecting to Redis", "err", err.Error())
		return
	}

	log.Info("Connected to Redis", "pong", pong)

	log.Info("Creating Slack client")
	slackAPI, err := slack.New(
		log.With("client", "slack"),
		os.Getenv("SLACK_TOKEN"),
	)
	if err != nil {
		log.Error("creating Slack client", "err", err.Error())
		return
	}

	channels, err := slackAPI.ListConversations()
	if err != nil {
		log.Error("listing Slack conversations", "err", err.Error())
		return
	}

	keys, err := rdb.Keys(ctx, "*").Result()
	if err != nil {
		log.Error("scanning Redis keys", "err", err.Error())
		return
	}

	threadRegexp := regexp.MustCompile(`\d*\.\d*`)
	for _, eventID := range keys {
		if threadRegexp.MatchString(eventID) {
			// We found a thread key, skip it
			continue
		}

		timestamp, err := rdb.Get(ctx, eventID).Result()
		if err != nil {
			log.Error(fmt.Sprintf("using the key(%s) listed with keys returns an error", eventID), "err", err.Error())
		}

		payload, err := rdb.Get(ctx, timestamp).Bytes()
		if err != nil {
			if err == redis.Nil {
				log.Info("Payload not found in Redis, deleting key", "key", timestamp)

				if err := rdb.Del(ctx, eventID).Err(); err != nil {
					log.Error("deleting key from Redis", "key", eventID, "err", err.Error())
					continue
				}

				continue
			}

			log.Error("getting message payload from Redis", "key", eventID, "err", err.Error())
			return
		}

		var message slack.Message
		if err := json.Unmarshal(payload, &message); err != nil {
			log.Error("unmarshalling message payload", "key", eventID, "err", err.Error())
			if err := rdb.Del(ctx, eventID).Err(); err != nil {
				log.Error("deleting key from Redis", "key", eventID, "err", err.Error())
				continue
			}
			continue
		}

		channel, ok := channels[message.Channel]
		if !ok {
			log.Info("channel not found in Slack conversations", "channel", message.Channel)
			// TODO: Delete the key if channel is not found?
			continue
		}

		var team string
		if org == "nais" {
			if strings.HasPrefix(channel, "nais-") {
				team = org
			} else {
				log.Info("Skipping message in non-nais channel", "channel", channel, "org", org)
				continue
			}
		} else {
			if strings.HasPrefix(channel, "nais-") {
				log.Info("Skipping message in nais-* channel", "channel", channel, "org", org)
				continue
			} else {
				team = findTeamByChannel(channel, teams)
				if team == "" {
					team = findTeamByChannel(message.Channel, teams)
					if team == "" {
						log.Warn("team not found for channel", "channel", channel, "key", eventID)
						continue
					}
				}
			}
		}

		if err := db.CreateSlackMessage(ctx, gensql.CreateSlackMessageParams{
			TeamSlug: team,
			EventID:  eventID,
			ThreadTs: timestamp,
			Channel:  message.Channel,
			Payload:  payload,
		}); err != nil {
			log.Error("inserting message into PostgreSQL", "key", eventID, "err", err.Error(), "team", team, "channel", channel, "timestamp", timestamp)
			continue
		}

		if err := rdb.Del(ctx, eventID).Err(); err != nil {
			log.Error("deleting key from Redis", "key", eventID, "err", err.Error())
			continue
		}

		log.Info("Successfully migrated key", "key", eventID, "thread_ts", timestamp, "channel", channel, "team", team)
	}
}

func findTeamByChannel(channel string, teams map[string]github.Team) string {
	// Remove the leading hash if it exists
	channel = strings.TrimPrefix(channel, "#")
	for _, team := range teams {
		if team.SlackChannels.Commits == channel || team.SlackChannels.Releases == channel || team.SlackChannels.Issues == channel || team.SlackChannels.PullRequests == channel || team.SlackChannels.Workflows == channel || team.Config.ExternalContributorsChannel == channel || team.SlackChannels.Security == channel {
			return team.Name
		}
	}

	// Maybe the channel is prefixed with a hash, try to find it again
	channel = "#" + channel
	for _, team := range teams {
		if team.SlackChannels.Commits == channel || team.SlackChannels.Releases == channel || team.SlackChannels.Issues == channel || team.SlackChannels.PullRequests == channel || team.SlackChannels.Workflows == channel || team.Config.ExternalContributorsChannel == channel || team.SlackChannels.Security == channel {
			return team.Name
		}
	}

	return ""
}
