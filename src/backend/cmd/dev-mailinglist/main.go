package main

import (
	"autocontract/pkg/mailinglist"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	envMailingListPath := flag.String("mailinglist-file", "", "the file to which emails from users will be appended to")
	envMailingListPrivateKey := flag.String("mailinglist-private-key-file", "", "the Base-64 encoded private key to decrypt mailing list entries with")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	reader, err := mailinglist.NewReader(*envMailingListPath, *envMailingListPrivateKey)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create mailinglist reader")
	}

	emails, err := reader.ReadAll()
	if err != nil {
		log.Fatal().Err(err).Msg("reading mailinglist failed")
	}

	var sb strings.Builder
	for _, email := range emails {
		sb.WriteString(email)
		sb.WriteString("\n")
	}
	allEmailsStr := sb.String()
	fmt.Printf("%s", allEmailsStr)
}
