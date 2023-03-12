package twitter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dghubble/oauth1"
	"github.com/g8rswimmer/go-twitter/v2"
)

type authorize struct{}

func (a authorize) Add(req *http.Request) {}

type Twtr struct {
	client *twitter.Client
}

func NewClient(consumerKey, consumerSecret, accessToken, accessTokenSecret string) *Twtr {
	conf := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := conf.Client(oauth1.NoContext, token)
	c := &twitter.Client{
		Authorizer: authorize{},
		Client:     httpClient,
		Host:       "https://api.twitter.com",
	}
	return &Twtr{
		c,
	}
}

func (t *Twtr) FindUserIdByScreenName(ctx context.Context, screenName string) (string, error) {
	res, err := t.client.UserNameLookup(ctx, []string{screenName}, twitter.UserLookupOpts{})
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	user := res.Raw.Users[0]
	return user.ID, nil
}

func (t *Twtr) BuildFetchMediaUrls(userId string) func(ctx context.Context) ([]string, bool, error) {
	paginationToken := ""
	return func(ctx context.Context) ([]string, bool, error) {
		opts := twitter.UserTweetTimelineOpts{
			PaginationToken: paginationToken,
			Expansions: []twitter.Expansion{
				twitter.ExpansionAttachmentsMediaKeys,
			},
			TweetFields: []twitter.TweetField{
				twitter.TweetFieldCreatedAt,
			},
			MediaFields: []twitter.MediaField{
				twitter.MediaFieldURL,
				twitter.MediaFieldMediaKey,
				twitter.MediaFieldVariants,
			},
			MaxResults: 100,
		}

		res, err := t.client.UserTweetTimeline(ctx, userId, opts)
		if err != nil {
			return nil, false, fmt.Errorf("fetch timeline: %w", err)
		}

		mediaUrls := []string{}
		if res.Raw.Includes != nil {
			media := make(map[string]string, len(res.Raw.Includes.Media))
			for _, m := range res.Raw.Includes.Media {
				switch m.Type {
				case "photo", "git":
					media[m.Key] = m.URL
					continue

				case "video":
					maxBitrateVariant := m.Variants[0]
					for _, v := range m.Variants {
						if v.BitRate > maxBitrateVariant.BitRate {
							maxBitrateVariant = v
						}
					}
					media[m.Key] = maxBitrateVariant.URL
					continue
				}
			}

			for _, tweet := range res.Raw.Tweets {
				if tweet.Attachments == nil {
					continue
				}
				// sort by index desc because tweets are ordered by reverse chronological
				picCount := len(tweet.Attachments.MediaKeys)
				reversedUrls := make([]string, picCount)
				for i, id := range tweet.Attachments.MediaKeys {
					reversedUrls[picCount-i-1] = media[id]
				}
				mediaUrls = append(mediaUrls, reversedUrls...)
			}
		}

		paginationToken = res.Meta.NextToken

		return mediaUrls, paginationToken != "", nil
	}
}
