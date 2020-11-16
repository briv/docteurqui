package doctorsearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/rs/zerolog/log"
)

var (
	ErrTemporarilyUnavailable = errors.New("busy creating index")
	ErrInvalidUserQuery       = errors.New("invalid user query")
)

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

var removeAccentsTransformer = transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)

type DoctorRecord struct {
	RPPSNumber string `json:"rpps"`
	FullName   string `json:"name"`
	Address    string `json:"address"`
}

type DoctorSearcher interface {
	Query(context.Context, string, int) ([]DoctorRecord, error)
	QueryTimeout() time.Duration
}

type drSearcher struct {
	indexControl       indexControl
	dataFilePath       string
	nGramSize          int
	maxUserQueryLength int
	maxQueryDuration   time.Duration
}

// New returns a DoctorSearcher capable of servicing user queries.
//
// Note that maxUserQueryLength is measured in bytes (and not in runes i.e characteres).
func New(rawDataFilePath string, nGramSize int, maxUserQueryLength int, maxConcurrentQueries int, maxQueryDuration time.Duration, indexUpdatePeriod time.Duration, indexUpdateMinPeriod time.Duration, indexUpdatePeriodJitter float32) DoctorSearcher {
	dr := &drSearcher{
		indexControl:       NewIndexControl(maxConcurrentQueries),
		dataFilePath:       filepath.Clean(rawDataFilePath),
		nGramSize:          nGramSize,
		maxUserQueryLength: maxUserQueryLength,
		maxQueryDuration:   maxQueryDuration,
	}

	// Launch background worker that attempts to create an index straight away,
	// from an existing data file.
	// The worker then runs the index update loop.
	go func() {
		index, err := buildIndex(dr.dataFilePath, dr.nGramSize)

		firstUpdate := NormalUpdate
		if err != nil {
			firstUpdate = FastUpdate
		} else {
			dr.indexControl.UseIndex(index)
		}

		if indexUpdatePeriod != 0 {
			iu := indexUpdater{
				updatePeriod:       indexUpdatePeriod,
				updateMinPeriod:    indexUpdateMinPeriod,
				updatePeriodJitter: indexUpdatePeriodJitter,
			}
			iu.Start(firstUpdate, dr)
		}
	}()

	return dr
}

func (dr drSearcher) QueryTimeout() time.Duration {
	return dr.maxQueryDuration
}

func (dr drSearcher) Query(ctx context.Context, unsafeUserQuery string, maxNumberResults int) ([]DoctorRecord, error) {
	start := time.Now()
	defer func() {
		log.Trace().
			Str("query", unsafeUserQuery).
			Dur("duration", time.Since(start)).
			Msg("doctor query")
	}()

	if len(unsafeUserQuery) > dr.maxUserQueryLength {
		return nil, fmt.Errorf("%w, query length %d exceeds limit %d", ErrInvalidUserQuery, len(unsafeUserQuery), dr.maxUserQueryLength)
	}

	// Check that our user query is valid UTF-8
	if utf8.ValidString(unsafeUserQuery) == false {
		return nil, fmt.Errorf("%w, query (%d bytes) was not valid utf-8", ErrInvalidUserQuery, len(unsafeUserQuery))
	}

	normalizedNoAccentQuery, _, err := transform.String(removeAccentsTransformer, unsafeUserQuery)
	if err != nil {
		return nil, fmt.Errorf("%w, query normalization and accent removal failed", ErrInvalidUserQuery)
	}

	if utf8.RuneCountInString(normalizedNoAccentQuery) < dr.nGramSize {
		return nil, fmt.Errorf("%w, minimum query length is %d", ErrInvalidUserQuery, dr.nGramSize)
	}

	// Limit the number of concurrent queries.
	storedIndex, err := dr.indexControl.Acquire(ctx)
	if err != nil {
		return nil, ErrTemporarilyUnavailable
	}
	defer dr.indexControl.Release(storedIndex)

	if storedIndex == nil {
		return nil, ErrTemporarilyUnavailable
	}

	records, err := storedIndex.query(ctx, normalizedNoAccentQuery, maxNumberResults)
	if err != nil {
		return nil, err
	}

	results := make([]DoctorRecord, len(records.orderedRecords))
	for i, rec := range records.orderedRecords {
		result := DoctorRecord{
			RPPSNumber: rec.RPPS(),
			FullName:   rec.FullName(),
			Address:    rec.Address(),
		}
		results[i] = result
	}

	return results, nil
}

// buildIndex creates an index using the data file at path dataFilePath.
func buildIndex(dataFilePath string, nGramSize int) (*nGramsIndex, error) {
	databaseFile, err := os.Open(dataFilePath)
	if err != nil {
		log.Error().Msgf("error opening index data file %s", err)
		return nil, err
	}

	start := time.Now()
	index, err := newNGramsIndex(databaseFile, nGramSize)
	if err != nil {
		log.Error().Msgf("error creating index %s", err)
		return nil, err
	}

	log.Info().
		Dur("create_duration", time.Since(start)).
		Int("records", index.numRecords).
		Int("index_entries", len(index.underlyingIndex)).
		Msg("created doctor search index")

	return index, nil
}
