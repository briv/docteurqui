package main

import (
	"autocontract/pkg/doctorsearch"
	"bufio"
	"context"
	"flag"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	drDataFilePath := flag.String("dr-data-file", "", "the file containing the doctor contact data. This should be an extraction from https://annuaire.sante.fr/web/site-pro/extractions-publiques")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	const (
		DoctorSearchNGramSize            = 3
		MaxDoctorSearchQueryLength       = 300
		MaxDoctorSearchConcurrentQueries = 200

		MaxNumberResults = 25
	)
	searcher := doctorsearch.New(*drDataFilePath, DoctorSearchNGramSize, MaxDoctorSearchQueryLength, MaxDoctorSearchConcurrentQueries)

	log.Debug().Msg("Starting...\n")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		ctx := context.Background()

		start := time.Now()
		results, err := searcher.Query(ctx, input, MaxNumberResults)
		queryDuration := time.Since(start)

		if err != nil {
			log.Error().Msgf("query error %v", err)
			continue
		}

		log.Debug().
			Str("query", input).
			Dur("dur", queryDuration).
			Int("num_results", len(results)).
			Msgf("query done:\n%v", results)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Uh uh, an error ! %v\n", err)
		os.Exit(1)
	}
}
