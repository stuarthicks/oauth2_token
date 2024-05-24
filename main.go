package main // import "github.com/stuarthicks/oauth2_token"

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
)

type Config struct {
	Clients map[string]Client `toml:"client"`
}

type Client struct {
	Base   string `toml:"base"`
	Id     string `toml:"id"`
	Secret string `toml:"secret"`
}

type ClientCredentials struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

var (
	config     Config
	configFile string
	clientName string
	printToken bool
)

func main() {
	flag.StringVar(&configFile, "f", "", "Path to config file")
	flag.StringVar(&clientName, "c", "", "Oauth client name in config file")
	flag.BoolVar(&printToken, "p", false, "Only print access token")
	flag.Parse()

	if configFile == "" {
		home, _ := homedir.Dir()
		configFile = filepath.Join(home, ".oauth.toml")
	}

	if clientName == "" {
		slog.Error("please specify a client name from the config file (see -h)")
		os.Exit(1)
	}

	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		slog.Error(
			"failed to parse config file",
			"config_file", configFile,
			"error", err.Error(),
		)
		os.Exit(1)
	}

	client, ok := config.Clients[clientName]
	if !ok {
		slog.Error(
			"client name not found in config file",
			"client_name", clientName,
			"config_file", configFile,
		)
		os.Exit(1)
	}

	var cacheFile = getCacheFilePath(client.Base, client.Id)

	if cacheExpired(cacheFile) {
		var data = url.Values{}
		data.Set("grant_type", "client_credentials")

		req, err := http.NewRequest(http.MethodPost, client.Base, strings.NewReader(data.Encode()))
		if err != nil {
			slog.Error(
				"failed to create http request",
				"error", err.Error(),
			)
			os.Exit(1)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(client.Id+":"+client.Secret)))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error(
				"failed to perform http request",
				"error", err.Error(),
			)
			os.Exit(1)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error(
				"failed to read response body",
				"error", err.Error(),
			)
			os.Exit(1)
		}

		if resp.StatusCode != http.StatusOK {
			slog.Error(
				"failed to obtain oauth access token",
				"status_code", resp.StatusCode,
				"response_body", string(respBody),
			)
			os.Exit(1)
		}

		f, err := os.Create(cacheFile)
		if err != nil {
			slog.Error(
				"failed to write cache file",
				"file", f,
				"error", err.Error(),
			)
			os.Exit(1)
		}
		_, _ = f.Write(respBody)
	}

	bs, err := os.ReadFile(cacheFile)
	if err != nil {
		slog.Error(
			"failed to read cache file",
			"file", cacheFile,
			"error", err.Error(),
		)
		os.Exit(1)
	}

	if !printToken {
		fmt.Println(string(bs))
		os.Exit(0)
	}

	var credentials ClientCredentials
	if err := json.NewDecoder(bytes.NewBuffer(bs)).Decode(&credentials); err != nil {
		slog.Error(
			"failed to decode client credentials",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	fmt.Println(credentials.AccessToken)
	os.Exit(0)
}

// cacheFilename normalises the endpoint and clientID into a stable "key" that is used to locate
// a cache file that contains the previous token for this client.
func cacheFilename(endpoint, clientID string) string {
	var s = endpoint + clientID
	return base64.URLEncoding.EncodeToString([]byte(s))
}

// getCacheFilePath returns the full path to a cache file. Respects XDG_CACHE_HOME if set.
func getCacheFilePath(endpoint, clientID string) string {
	var cacheDir = os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, _ := homedir.Dir()
		cacheDir = filepath.Join(home, ".cache")
	}
	var oauth2TokenCacheDir = filepath.Join(cacheDir, "oauth2_token")
	_ = os.MkdirAll(oauth2TokenCacheDir, 0750)

	var oauth2TokenCacheFile = cacheFilename(endpoint, clientID) + ".json"
	return filepath.Join(oauth2TokenCacheDir, oauth2TokenCacheFile)
}

// cacheExpired checks the `expires` in a cache file to determine if the file has expired or not.
func cacheExpired(f string) bool {
	bs, err := os.ReadFile(f)
	if err != nil {
		return true // if we couldn't read the file, then it probably needs to be created
	}
	var credentials ClientCredentials
	if err := json.NewDecoder(bytes.NewBuffer(bs)).Decode(&credentials); err != nil {
		return true // if we couldn't parse the file, then let's just blat it with valid creds
	}
	finfo, err := os.Stat(f)
	if err != nil {
		return true // if we couldn't stat the file, then it probably needs to be created/blatted
	}

	if time.Now().After(finfo.ModTime().Add(time.Duration(credentials.ExpiresIn*int(time.Second)) - (10 * time.Second))) {
		return true // the current time is past the expiry time (with a 10 second buffer), we should get new creds
	}

	return false
}
