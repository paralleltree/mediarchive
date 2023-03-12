package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/dghubble/oauth1"
	twauth "github.com/dghubble/oauth1/twitter"
	"github.com/paralleltree/mediarchive/config"
	"github.com/paralleltree/mediarchive/logger"
	"github.com/paralleltree/mediarchive/twitter"
	"github.com/urfave/cli/v2"
)

func main() {
	logger := logger.NewLogger()
	defaultConfigPath := ResolveDefaultConfigPath()
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       defaultConfigPath,
				DefaultText: "`settings.yml` in executable directory",
			},
		},
		Commands: []*cli.Command{
			{
				Name: "auth-twitter",
				Action: func(c *cli.Context) error {
					appConfig, err := config.LoadConfig(c.String("config"))
					if err != nil {
						return fmt.Errorf("load config: %w", err)
					}
					conf := oauth1.Config{
						ConsumerKey:    appConfig.Twitter.ConsumerKey,
						ConsumerSecret: appConfig.Twitter.ConsumerSecret,
						CallbackURL:    "oob",
						Endpoint:       twauth.AuthorizeEndpoint,
					}

					requestToken, _, err := conf.RequestToken()
					if err != nil {
						return fmt.Errorf("request token: %w", err)
					}

					authorizeUrl, err := conf.AuthorizationURL(requestToken)
					if err != nil {
						return fmt.Errorf("get authorization url: %w", err)
					}
					fmt.Printf("Open this url: %s\n", authorizeUrl)
					fmt.Printf("Enter the PIN: ")
					var pin string
					if _, err := fmt.Scanf("%s", &pin); err != nil {
						return fmt.Errorf("scan: %w", err)
					}

					accessToken, accessSecret, err := conf.AccessToken(requestToken, "", pin)
					if err != nil {
						return fmt.Errorf("get access token: %w", err)
					}

					fmt.Printf("access token: %s\n", accessToken)
					fmt.Printf("access secret: %s\n", accessSecret)

					return nil
				},
			},
			{
				Name: "collect-twitter",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "screen-name",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "dest-dir",
						Value: ".",
					},
					&cli.BoolFlag{
						Name:  "overwrite",
						Value: false,
					},
				},
				Action: func(c *cli.Context) error {
					ctx := c.Context

					appConfig, err := config.LoadConfig(c.String("config"))
					if err != nil {
						return fmt.Errorf("load config: %w", err)
					}
					destDir := c.String("dest-dir")
					buildDestPath := func(u *url.URL) string {
						name := path.Base(u.Path)
						return filepath.Join(destDir, name)
					}

					saver := buildDownloader(logger, buildDestPath, c.Bool("overwrite"))
					stopPredicate := func(u *url.URL) bool {
						dest := buildDestPath(u)
						return fileExists(dest)
					}

					client := twitter.NewClient(appConfig.Twitter.ConsumerKey, appConfig.Twitter.ConsumerSecret, appConfig.Twitter.AccessKey, appConfig.Twitter.AccessSecret)
					id, err := client.FindUserIdByScreenName(ctx, c.String("screen-name"))
					if err != nil {
						return fmt.Errorf("find user: %w", err)
					}
					fetcher := client.BuildFetchMediaUrls(id)

					if err := fetchUntilThenProcessInReversedOrder(ctx, fetcher, stopPredicate, saver); err != nil {
						return fmt.Errorf("processing: %w", err)
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("%v", err)
	}
}

// iterate fetching over reverse chronological order and returns chronological result
func fetchUntilThenProcessInReversedOrder(
	ctx context.Context,
	fetch func(ctx context.Context) ([]string, bool, error),
	stopPredicate func(u *url.URL) bool,
	process func(ctx context.Context, u *url.URL) error,
) error {
	mediaUrls := []*url.URL{}
	stop := false
	for !stop {
		urls, hasNext, err := fetch(ctx)
		if err != nil {
			return fmt.Errorf("fetch media: %w", err)
		}
		stop = stop || !hasNext

		for _, u := range urls {
			u, err := url.Parse(u)
			if err != nil {
				return fmt.Errorf("parse url: %w", err)
			}
			if stopPredicate(u) {
				stop = true
				break
			}
			mediaUrls = append(mediaUrls, u)
		}
	}

	for i := 0; i < len(mediaUrls); i++ {
		// process media in reversed order
		if err := process(ctx, mediaUrls[len(mediaUrls)-i-1]); err != nil {
			return fmt.Errorf("process media: %w", err)
		}
		time.Sleep(time.Second)
	}

	return nil
}

func buildDownloader(logger logger.Logger, buildDest func(u *url.URL) string, overwrite bool) func(ctx context.Context, u *url.URL) error {
	return func(ctx context.Context, u *url.URL) error {
		dest := buildDest(u)

		if fileExists(dest) && !overwrite {
			logger.Info("file %s already exists. skipping.\n", dest)
			return nil
		}

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return fmt.Errorf("build http request: %w", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("do http request: %w", err)
		}
		defer res.Body.Close()

		f, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		defer f.Close()

		if _, err := io.Copy(f, res.Body); err != nil {
			return fmt.Errorf("copy stream: %w", err)
		}

		logger.Info("%s downloaded.\n", u.String())
		return nil
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ResolveDefaultConfigPath() string {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dir := filepath.Dir(executable)
	return filepath.Join(dir, "settings.yml")
}
