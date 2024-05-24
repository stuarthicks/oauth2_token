# oauth2_token

Convenience wrapper for getting oauth2 tokens to use with other tools, eg. curl.

## Install

Using Homebrew:

    brew install stuarthicks/tap/oauth2_token

Using Go:

    go install github.com/stuarthicks/oauth2_token

## Usage

Copy the example oauth config to `~/.oauth.toml`.

Raw oauth2 response:

    # oauth2_token -c "{client}" -d | jq
    {
      "access_token": "hunter2",
      "expires_in": 300,
      "token_type": "Bearer"
    }

Use in curl:

    # curl --oauth2-bearer "$(oauth2_token -c "{client}")" https://api.foo.example.com/
