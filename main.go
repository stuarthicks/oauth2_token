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

	if !printToken {
		fmt.Println(string(respBody))
		os.Exit(0)
	}

	var credentials ClientCredentials
	if err := json.NewDecoder(bytes.NewBuffer(respBody)).Decode(&credentials); err != nil {
		slog.Error(
			"failed to decode client credentials",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	fmt.Println(credentials.AccessToken)
	os.Exit(0)
}
