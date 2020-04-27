package doctorsearch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
)

type ReadSeekerCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type queryOrderableRecordReadWish struct {
	Offset DatabaseFileOffsetsRecord
	Count  int
}

type readWishResult struct {
	record *rawPersonActivityRecord
	err    error
}

type readRecordsCom struct {
	readWishes         []queryOrderableRecordReadWish
	readRecordsChan    chan *rawPersonActivityRecord
	readRecordsErrChan chan error
}

type nGramsIndex struct {
	nGramSize        int
	underlyingIndex  map[string][]DatabaseFileOffsetsRecord // TODO: use [NGramSize]rune for key instead ?
	recordsData      ReadSeekerCloser
	submitReadsQueue chan readRecordsCom
	done             chan struct{} //
}

func newNGramsIndex(r ReadSeekerCloser, nGramSize int) (*nGramsIndex, error) {
	scanner := bufio.NewScanner(r)

	index := make(map[string][]DatabaseFileOffsetsRecord, 0)
	var (
		offset      int64 = 0
		lastAdvance int64 = 0
	)
	// Store the used RPPS among query results as the file contains duplicates
	usedRPPSMap := make(map[string]bool)

	split := func() bufio.SplitFunc {
		hasReadHeader := false

		return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if hasReadHeader == false {
				advanceHeader, _, errHeader := bufio.ScanLines(data, atEOF)
				if errHeader != nil {
					return advanceHeader, nil, errHeader
				}
				hasReadHeader = true
				lastAdvance = int64(advanceHeader)
				offset += lastAdvance
				return advanceHeader, nil, nil
			}

			advance, token, err = bufio.ScanLines(data, atEOF)
			if err == nil && token != nil {
				_, err = parseRecordFromLine(string(token))
			}
			lastAdvance = int64(advance)
			offset += lastAdvance
			return
		}
	}()
	// Set the split function for the scanning operation.
	scanner.Split(split)

	for scanner.Scan() {
		line := scanner.Text()

		record, err := parseRecordFromLine(line)
		if err != nil {
			return nil, err
		}
		if record.shouldBeIndexed() == false {
			continue
		}
		rpps := record.RPPS()
		if _, alreadyPresent := usedRPPSMap[rpps]; alreadyPresent {
			// TODO: be smarter about which record we keep out of the ones with the same RPPS ?
			continue
		}
		usedRPPSMap[rpps] = true

		ngrams := record.computeAllNGrams(nGramSize)
		for _, ngram := range ngrams {
			existingOffsets, ok := index[ngram]
			if !ok {
				existingOffsets = make([]DatabaseFileOffsetsRecord, 0)
			}
			// `offset` is the current offset after reading the record, so we substract
			// the number of bytes that were just read (`lastAdvance`).
			actualOffset := offset - lastAdvance

			index[ngram] = insertOffset(DatabaseFileOffsetsRecord{
				StartOffset: actualOffset,
				Length:      uint32(len(line)),
			}, existingOffsets)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	newGramsIndex := &nGramsIndex{
		nGramSize:        nGramSize,
		underlyingIndex:  index,
		recordsData:      r,
		submitReadsQueue: make(chan readRecordsCom),
		done:             make(chan struct{}, 1),
	}
	go newGramsIndex.readServiceWorker()
	return newGramsIndex, nil
}

type byHitCount []queryOrderableRecordReadWish

func (a byHitCount) Len() int           { return len(a) }
func (a byHitCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byHitCount) Less(i, j int) bool { return a[i].Count > a[j].Count }

type queryResult struct {
	orderedRecords []rawPersonActivityRecord
}

// query returns the records
func (ngi *nGramsIndex) query(ctx context.Context, query string, maxNumberResults int) (queryResult, error) {
	// For user queries, compute the union of ngrams of all (space-padded) words in the query,
	// e.g. for query "dorier marina", use the union of ngrams from " dorier " and " marina ".
	queryTokens := strings.Split(query, " ")

	queryNgrams := make(map[string]bool)
	// 1. divide query into all possible ngrams, of length N.
	for _, queryToken := range queryTokens {
		ngms := ngrams(fmt.Sprintf(" %s ", strings.ToLower(queryToken)), ngi.nGramSize)
		for _, ngm := range ngms {
			queryNgrams[ngm] = true
		}
	}

	// 2. for each above ngram, get possible record offsets
	resultsCount := make(map[int64]int)
	resultsValues := make(map[int64]DatabaseFileOffsetsRecord)
	for queryNgram, _ := range queryNgrams {
		offsets := ngi.underlyingIndex[queryNgram]

		for _, offset := range offsets {
			resultsCount[offset.StartOffset] = resultsCount[offset.StartOffset] + 1
			resultsValues[offset.StartOffset] = offset
		}
	}

	// 3. construct an ordered list of: (offset, count / Nq )
	results := make([]queryOrderableRecordReadWish, len(resultsCount))
	var i int
	for offset, count := range resultsCount {
		results[i] = queryOrderableRecordReadWish{
			Offset: resultsValues[offset],
			Count:  count,
		}
		i++
	}
	sort.Sort(byHitCount(results))

	log.Trace().Msgf("query results: %v", len(results))

	// Check after our in-memory computation phase that we're still ok to continue.
	if err := ctx.Err(); err != nil {
		return queryResult{}, err
	}

	// 4. Grab the top N (at most) ranked records by seeking in the file to the offset
	// and reading off all the necessary data.
	if len(results) > maxNumberResults {
		results = results[:maxNumberResults]
	}
	records, err := ngi.readRecords(ctx, results)
	if err != nil {
		return queryResult{}, err
	}

	// 5. Possibly limit the top ranked records by requiring them to have a minimum probability (0.3 coming from postgresql could be a good start ?) or maybe use an edit distance calculation to prune the results further after this ??

	// 6. Possibly re-rank any ex-aequo records by using edit distance between query and record values ?
	// To go around name/lastname issues, perhaps try all edit distances between the 4 pairs of (query token, lastname | name )
	// and pick the best scoring pairing ?
	return queryResult{
		orderedRecords: records,
	}, nil
}

func (ngi *nGramsIndex) Close() {
	// Give a certain amount of time for any currently in flight reads to be serviced.
	gracefulTimePeriod := 30 * time.Second
	// This is more than enough as each query should have its own deadline which
	// ensures it won't be running for this long.
	time.AfterFunc(gracefulTimePeriod, func() {
		ngi.done <- struct{}{}
	})
}

func (ngi *nGramsIndex) readServiceWorker() {
	defer ngi.recordsData.Close()

	// 2kB should be fine for most records.
	const BufferSize = 2 * 1024
	b := make([]byte, BufferSize)

	for {
		select {
		case <-ngi.done:
			return
		case com := <-ngi.submitReadsQueue:
			// A new wish for reads has come in.
			// Loop over them and seek to the correct position to read out the data.
			for _, readWish := range com.readWishes {
				_, err := ngi.recordsData.Seek(readWish.Offset.StartOffset, io.SeekStart)
				// If an error occurs, stop immediately as we want all records or none.
				if err != nil {
					com.readRecordsErrChan <- err
					break
				}

				// Avoid allocating a buffer each time and reuse the one we have.
				if uint32(cap(b)) >= readWish.Offset.Length {
					b = b[:readWish.Offset.Length]
				} else {
					b = make([]byte, readWish.Offset.Length)
				}

				_, err = io.ReadFull(ngi.recordsData, b)
				if err != nil {
					com.readRecordsErrChan <- err
					break
				}

				line := string(b)
				record, err := parseRecordFromLine(line)
				if err != nil {
					com.readRecordsErrChan <- err
					break
				}

				com.readRecordsChan <- record
			}
		}
	}
}

func (ngi *nGramsIndex) readRecords(ctx context.Context, readWishes []queryOrderableRecordReadWish) ([]rawPersonActivityRecord, error) {
	records := make([]rawPersonActivityRecord, len(readWishes))

	// It's important for these channels to have sufficient buffering to avoid
	// blocking the worker which will be servicing these channels.
	readResultsChan := make(chan *rawPersonActivityRecord, len(readWishes))
	readResultsErrChan := make(chan error, 1)
	com := readRecordsCom{
		readWishes:         readWishes,
		readRecordsChan:    readResultsChan,
		readRecordsErrChan: readResultsErrChan,
	}

	var successfullyRead int
	for {
		select {
		case ngi.submitReadsQueue <- com:
			// This is the first operation that should succeed, allowing the records to be read.
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-readResultsErrChan:
			return nil, err
		case record := <-readResultsChan:
			records[successfullyRead] = *record
			successfullyRead += 1
			if successfullyRead >= len(readWishes) {
				return records, nil
			}
		}
	}
}

// Returns a list of ngrams of the given UTF-8 string.
//
// Note that non UTF-8 strings may return bizarre results.
func ngrams(s string, N int) (ngrams []string) {
	// Store the current ngram as a rune slice.
	ngram := make([]rune, 3)
	// Iterate over the string using byte indexing
	for i, w := 0, 0; i < len(s); i += w {
		// Sentinel value to check if we were able to decode the first rune and get its width.
		var firstRuneWidth int = -1
		nGramIsComplete := false
		for j, totalOffset := 0, 0; j < N; j += 1 {
			nextRune, nextWidth := utf8.DecodeRuneInString(s[i+totalOffset:])
			// If there is an error while retrieving the next character in the current ngram, just abort.
			if nextRune == utf8.RuneError {
				break
			}
			ngram[j] = nextRune
			totalOffset += nextWidth
			if j == 0 {
				firstRuneWidth = nextWidth
			} else if j == N-1 {
				nGramIsComplete = true
			}
		}

		if nGramIsComplete {
			ngrams = append(ngrams, string(ngram))
		}

		if firstRuneWidth == -1 {
			// There was an error decoding the first rune, advance by just one byte.
			firstRuneWidth = 1
		}
		w = firstRuneWidth
	}

	return
}

type DatabaseFileOffsetsRecord struct {
	StartOffset int64
	Length      uint32
}

func insertOffset(offset DatabaseFileOffsetsRecord, sortedOffsets []DatabaseFileOffsetsRecord) []DatabaseFileOffsetsRecord {
	s := sortedOffsets
	mid := len(s) / 2
	switch {
	case len(s) == 0:
		{
			return append(s, offset)
		}
	case s[mid].StartOffset > offset.StartOffset:
		{
			begin := insertOffset(offset, s[:mid])
			return append(begin, s[mid:]...)
		}
	case s[mid].StartOffset < offset.StartOffset:
		{
			end := insertOffset(offset, s[mid+1:])
			return append(s[:mid+1], end...)
		}
	default: // s[mid] == offset
		// No duplicates, so don't insert the value.
		return s
	}
}

type rawPersonActivityRecord struct {
	PPIdType                     uint8  // 0
	PPId                         string // 1
	Nom                          string // 7
	Prenom                       string // 8
	CodeProfession               string // 9  e.g. "10" for a doctor
	LibelleProfession            string // 10 e.g. "Medecin"
	CodeCategorieProfessionnelle string // 11 e.g. "M" for "Militaire"
	CodeModeExercice             string // 17 e.g. "L" for "Liberal"
	NumeroVoie                   string // 28 e.g. "68"
	IndiceRepetitionVoie         string // 29 e.g. "bis"
	LibelleTypeDeVoie            string // 31 e.g. "rue" or "avenue"
	LibelleVoie                  string // 32 e.g. "des Lilas"
	CodePostal                   string // 35 e.g. "75016"
	LibelleCommune               string // 37 e.g. "Paris"
}

func (rec *rawPersonActivityRecord) shouldBeIndexed() bool {
	// Check that our ID is of type "RPPS" (i.e 8), that the profession is "Doctor" (i.e 10)
	// and that the "exercise mode" is "Libéral" (i.e "L")
	if rec.PPIdType == 8 && rec.CodeProfession == "10" && rec.CodeModeExercice == "L" {
		// We're quite trusting on the other fields, but check that at least the name is valid utf-8.
		if utf8.ValidString(rec.Nom) && utf8.ValidString(rec.Prenom) {
			return true
		}
	}
	return false
}

func (rec *rawPersonActivityRecord) RPPS() string {
	return rec.PPId
}

func (rec *rawPersonActivityRecord) FullName() string {
	firstName := strings.Title(strings.ToLower(rec.Prenom))
	lastName := strings.Title(strings.ToLower(rec.Nom))
	return fmt.Sprintf("%s %s", firstName, lastName)
}

func (rec *rawPersonActivityRecord) Address() string {
	libelleVoie := strings.TrimSpace(rec.LibelleVoie)
	codePostal := strings.TrimSpace(rec.CodePostal)
	libelleCommune := strings.TrimSpace(rec.LibelleCommune)

	// We consider a useful address must have at least the following 3 items.
	if libelleVoie == "" || codePostal == "" || libelleCommune == "" {
		return ""
	}

	didWriteStreetNumber := false

	var sb strings.Builder
	// Only include number if it is present.
	numeroVoie := strings.TrimSpace(rec.NumeroVoie)
	if numeroVoie != "" {
		sb.WriteString(numeroVoie)
		didWriteStreetNumber = true
	}
	// Only include bis/ter/... if present.
	indiceRepetitionVoie := strings.TrimSpace(rec.IndiceRepetitionVoie)
	if indiceRepetitionVoie != "" {
		sb.WriteString(strings.ToLower(indiceRepetitionVoie))
		didWriteStreetNumber = true
	}

	if didWriteStreetNumber {
		sb.WriteString(" ")
	}

	libelleTypeDeVoie := strings.TrimSpace(rec.LibelleTypeDeVoie)
	if libelleTypeDeVoie != "" {
		fmt.Fprintf(&sb, "%s ", strings.ToLower(libelleTypeDeVoie))
	}

	fmt.Fprintf(&sb, "%s, %s %s",
		strings.Title(strings.ToLower(libelleVoie)),
		codePostal,
		strings.ToUpper(libelleCommune),
	)
	address := sb.String()
	return address
}

func (rec *rawPersonActivityRecord) computeAllNGrams(N int) []string {
	gramsFirst := ngrams(fmt.Sprintf(" %s ", strings.ToLower(rec.Prenom)), N)
	gramsLast := ngrams(fmt.Sprintf(" %s ", strings.ToLower(rec.Nom)), N)
	return append(gramsFirst, gramsLast...)
}

func parseRecordFromLine(line string) (*rawPersonActivityRecord, error) {
	cols := strings.Split(line, "|")
	if len(cols) != 53 {
		return nil, fmt.Errorf("unexpected number of columns (%d)", len(cols))
	}
	ppIdType, err := strconv.ParseUint(cols[0], 10, 8)
	if err != nil {
		return nil, err
	}

	return &rawPersonActivityRecord{
		PPIdType:                     uint8(ppIdType),
		PPId:                         cols[1],
		Nom:                          cols[7],
		Prenom:                       cols[8],
		CodeProfession:               cols[9],
		LibelleProfession:            cols[10],
		CodeCategorieProfessionnelle: cols[11],
		CodeModeExercice:             cols[17],
		NumeroVoie:                   cols[28],
		IndiceRepetitionVoie:         cols[29],
		LibelleTypeDeVoie:            cols[31],
		LibelleVoie:                  cols[32],
		CodePostal:                   cols[35],
		LibelleCommune:               cols[37],
	}, nil
}
