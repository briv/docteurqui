package doctorsearch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sync/semaphore"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	TemporarilyUnavailable = errors.New("busy creating index")
	InvalidUserQuery       = errors.New("invalid user query")
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
}

type drSearcher struct {
	index              *atomic.Value // really an *nGramsIndex
	dataDirectory      string
	semWorkLimiter     *semaphore.Weighted
	nGramSize          int
	maxUserQueryLength int
}

// New returns a DoctorSearcher capable of servicing user queries.
//
// Note that maxUserQueryLength is measured in bytes (and not in runes i.e characteres).
func New(rawDataDir string, nGramSize int, maxUserQueryLength int, maxConcurrentQueries int) DoctorSearcher {
	semTotalWeight := int64(maxConcurrentQueries)
	s := &drSearcher{
		index:              &atomic.Value{},
		dataDirectory:      rawDataDir,
		semWorkLimiter:     semaphore.NewWeighted(semTotalWeight),
		nGramSize:          nGramSize,
		maxUserQueryLength: maxUserQueryLength,
	}
	// On creation, build search index to enable queries.
	go s.tryToRecreateIndex()
	return s
}

func (ds drSearcher) Query(ctx context.Context, unsafeUserQuery string, maxNumberResults int) ([]DoctorRecord, error) {
	start := time.Now()
	defer func() { log.Printf("Query took %s", time.Since(start)) }()

	if len(unsafeUserQuery) > ds.maxUserQueryLength {
		return nil, fmt.Errorf("%w, query length %d exceeds limit %d", InvalidUserQuery, len(unsafeUserQuery), ds.maxUserQueryLength)
	}

	// Check that our user query is valid UTF-8
	if utf8.ValidString(unsafeUserQuery) == false {
		return nil, fmt.Errorf("%w, query (%d bytes) was not valid utf-8", InvalidUserQuery, len(unsafeUserQuery))
	}

	normalizedNoAccentQuery, _, err := transform.String(removeAccentsTransformer, unsafeUserQuery)
	if err != nil {
		return nil, fmt.Errorf("%w, query normalization and accent removal failed", InvalidUserQuery)
	}

	if utf8.RuneCountInString(normalizedNoAccentQuery) < ds.nGramSize {
		return nil, fmt.Errorf("%w, minimum query length is %d", InvalidUserQuery, ds.nGramSize)
	}

	// Limit the number of concurrent queries.
	if err := ds.semWorkLimiter.Acquire(ctx, 1); err != nil {
		// log.Printf("Failed to acquire semaphore: %v", err)
		return nil, err
	}
	defer ds.semWorkLimiter.Release(1)
	// The call to Acquire() above can still succeed after our context is done.
	// Therefore, check the context error ourselves.
	if ctx.Err() != nil {
		return nil, TemporarilyUnavailable
	}

	storedIndex := ds.index.Load()
	if storedIndex == nil {
		return nil, TemporarilyUnavailable
	}

	index := storedIndex.(*nGramsIndex)
	records, err := index.query(ctx, normalizedNoAccentQuery, maxNumberResults)
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

func (ds *drSearcher) tryToRecreateIndex() {
	// TODO: remove this hard-coded value
	dataFilePath := filepath.Join(ds.dataDirectory, "PS_LibreAcces_Personne_activite_202003041023.txt")
	databaseFile, err := os.Open(dataFilePath)
	if err != nil {
		log.Fatalf("error opening data file %s \n", err)
		return
	}
	start := time.Now()
	index, err := newNGramsIndex(databaseFile, ds.nGramSize)
	if err != nil {
		log.Fatalf("error creating index %s \n", err)
		return
	}
	log.Printf("index creation (%d entries) took %s", len(index.underlyingIndex), time.Since(start))

	previousIndexInterface := ds.index.Load()
	if previousIndexInterface != nil {
		previousIndex := previousIndexInterface.(*nGramsIndex)
		// TOOD: make this safe
		defer previousIndex.Close()
	}

	ds.index.Store(index)
}
