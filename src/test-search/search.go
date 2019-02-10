package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func main() {

	_, err := os.Stat("./clean.txt")
	log.Printf("Checking 'clean.txt' file, err is: %v", err)
	if err != nil {
		file, err := os.Create("./clean.txt")
		if err != nil {
			log.Printf("error creating file %v\n", err)
			os.Exit(1)
		}

		rawDataFile, err := os.Open("./PS_LibreAcces_202003041402/PS_LibreAcces_Personne_activite_202003041023.txt")
		if err != nil {
			log.Printf("error opening file %v\n", err)
			os.Exit(1)
		}

		r := bufio.NewReader(rawDataFile)
		writer := bufio.NewWriter(file)
		err = filterAndWriteData(r, writer)
		if err != nil {
			log.Printf("error doing stuff %v\n", err)
			os.Exit(1)
		}

		rawDataFile.Close()
		file.Close()
	}

	databaseFile, err := os.Open("./clean.txt")
	if err != nil {
		log.Printf("error opening db file %v\n", err)
		os.Exit(1)
	}
	defer databaseFile.Close()

	const NGramSize = 3
	ngi, err := New(databaseFile, NGramSize)
	if err != nil {
		log.Printf("error creating n-gram index %v\n", err)
		os.Exit(1)
	}

	// for testing
	log.Printf("Ready !\n")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		fmt.Printf("query:'%s'\n", input)
		start := time.Now()
		ngi.Query(input)
		log.Printf("Query '%s' took %s", input, time.Since(start))
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Uh uh, an error ! %v\n", err)
		os.Exit(1)
	}
}

type DatabaseFileOffset struct {
	FileOffset int64
}

func insertOffset(sortedOffsets []DatabaseFileOffset, offset DatabaseFileOffset) []DatabaseFileOffset {
	s := sortedOffsets
	mid := len(s) / 2
	switch {
	case len(s) == 0:
		{
			return append(s, offset)
		}
	case s[mid].FileOffset > offset.FileOffset:
		{
			begin := insertOffset(s[:mid], offset)
			return append(begin, s[mid:]...)
		}
	case s[mid].FileOffset < offset.FileOffset:
		{
			end := insertOffset(s[mid+1:], offset)
			return append(s[:mid+1], end...)
		}
	default: // s[mid] == offset
		// No duplicates, so don't insert the value.
		return s
	}
}

type NGramsIndex struct {
	NGramSize   int
	index       map[string][]DatabaseFileOffset // TODO: use [NGramSize]rune for key instead ?
	recordsData io.ReadSeeker
}

func New(r io.ReadSeeker, NGramSize int) (*NGramsIndex, error) {
	scanner := bufio.NewScanner(r)

	index := make(map[string][]DatabaseFileOffset, 0)
	var (
		offset      int64 = 0
		lastAdvance int64 = 0
	)
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanLines(data, atEOF)
		if err == nil && token != nil {
			_, err = parseRecordFromLine(string(token))
		}
		lastAdvance = int64(advance)
		offset += lastAdvance
		return
	}
	// Set the split function for the scanning operation.
	scanner.Split(split)
	for scanner.Scan() {
		line := scanner.Text()

		record, err := parseRecordFromLine(line)
		if err != nil {
			return nil, err
		}
		ngrams := computeAllNGrams(record, NGramSize)
		// log.Printf("DEBUG OFFSET: %d | ngrams: %d\n", offset-lastAdvance, len(ngrams))
		for _, ngram := range ngrams {
			existingOffsets, ok := index[ngram]
			if !ok {
				existingOffsets = make([]DatabaseFileOffset, 0)
			}
			// `offset` is the current offset after reading the record, so we substract
			// the number of bytes that were just read (`lastAdvance`).
			actualOffset := offset - lastAdvance

			index[ngram] = insertOffset(existingOffsets, DatabaseFileOffset{
				FileOffset: actualOffset,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &NGramsIndex{
		NGramSize:   NGramSize,
		index:       index,
		recordsData: r,
	}, nil
}

type Result struct {
	Offset int64
	Count  int
}
type ByHitCount []Result

func (a ByHitCount) Len() int           { return len(a) }
func (a ByHitCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByHitCount) Less(i, j int) bool { return a[i].Count > a[j].Count }

type ByOffset []Result

func (a ByOffset) Len() int           { return len(a) }
func (a ByOffset) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByOffset) Less(i, j int) bool { return a[i].Offset < a[j].Offset }

func (ngi *NGramsIndex) Query(query string) ([]string, error) {
	// 0. TODO: if query has spaces, maybe use the union of ngrams for
	// all words, e.g. for query "dorier marina", use the union of ngrams from " dorier " and " marina "
	queryTokens := strings.Split(query, " ")

	queryNgrams := make(map[string]bool, 0)
	// 1. divide query into all possible ngrams, of length Nq
	for _, queryToken := range queryTokens {
		ngms := ngrams(fmt.Sprintf(" %s ", queryToken), ngi.NGramSize)
		for _, ngm := range ngms {
			queryNgrams[ngm] = true
		}
	}
	log.Println(queryNgrams)

	log.Printf("Index size %d\n", len(ngi.index))

	// 2. for each above ngram, get possible record offsets
	resultOffsets := make(map[int64]int)
	for queryNgram, _ := range queryNgrams {
		offsets := ngi.index[queryNgram]

		log.Printf("%d ", len(offsets))

		for _, offset := range offsets {
			resultOffsets[offset.FileOffset] = resultOffsets[offset.FileOffset] + 1
		}
	}

	// 3. construct an ordered list of: (offset, count / Nq )
	var results []Result
	for offset, count := range resultOffsets {
		results = append(results, Result{
			Offset: offset,
			Count:  count,
		})
	}
	sort.Sort(ByHitCount(results))

	log.Printf("Results are in: %v\n", len(results))

	// 4. Grab the top 3/4/5 ranked records by seeking in the file to the offset
	//    and reading off all the necessary data.
	const TopN = 5
	results = results[:TopN]

	// TODO: is this necessary: sort by Offset to help IO as we seek in the same direction ??
	sort.Sort(ByOffset(results))
	for _, result := range results {
		log.Printf("Offset/Count is: %v/%d\n", result.Offset, result.Count)

		_, err := ngi.recordsData.Seek(result.Offset, io.SeekStart)
		if err != nil {
			return nil, err
		}

		br := bufio.NewReader(ngi.recordsData)
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		record, err := parseRecordFromLine(line)
		if err != nil {
			return nil, err
		}
		log.Printf("Record is: %v\n", record)
	}

	// 5. Possibly limit the top ranked records by requiring them to have a minimum probability (0.3 coming from postgresql could be a good start ?) or maybe use an edit distance calculation to prune the results further after this ??

	// 6. Possibly re-rank any ex-aequo records by using edit distance between query and record values ?
	// To go around name/lastname issues, perhaps try all edit distances between the 4 pairs of (query token, lastname | name )
	// and pick the best scoring pairing ?
	return []string{}, nil
}

type RawPersonActivityRecord struct {
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

type PersonActivityRecord struct {
	NumberRPPS string
	Name       string
	LastName   string
	Address    string
	// NumberADELI string
}

func parseRecordFromLine(line string) (*PersonActivityRecord, error) {
	cols := strings.Split(line, "|")
	if len(cols) != 4 {
		return nil, fmt.Errorf("unexpected number of columns (%d)", len(cols))
	}
	// TODO: use utf8.ValidString to check validity of strings here
	return &PersonActivityRecord{
		NumberRPPS: cols[0],
		Name:       cols[1],
		LastName:   cols[2],
		Address:    cols[3],
	}, nil
}

func parseRawRecordFromLine(line string) (*RawPersonActivityRecord, error) {
	subs := strings.Split(line, "|")
	if len(subs) != 53 {
		return nil, fmt.Errorf("unexpected number of columns (%d) '%s'", len(subs), line)
	}
	ppIdType, err := strconv.ParseUint(subs[0], 10, 8)
	if err != nil {
		return nil, err
	}
	// TODO: use some sort of method (utf8.ValidString ?) to check validity of strings here
	return &RawPersonActivityRecord{
		PPIdType:                     uint8(ppIdType),
		PPId:                         subs[1],
		Nom:                          subs[7],
		Prenom:                       subs[8],
		CodeProfession:               subs[9],
		LibelleProfession:            subs[10],
		CodeCategorieProfessionnelle: subs[11],
		CodeModeExercice:             subs[17],
		NumeroVoie:                   subs[28],
		IndiceRepetitionVoie:         subs[29],
		LibelleTypeDeVoie:            subs[31],
		LibelleVoie:                  subs[32],
		CodePostal:                   subs[35],
		LibelleCommune:               subs[37],
	}, nil
}

func writeRecord(pa *RawPersonActivityRecord, writer io.Writer) (int, error) {
	var sb strings.Builder
	sb.WriteString(pa.NumeroVoie)
	if pa.IndiceRepetitionVoie != "" {
		sb.WriteString(pa.IndiceRepetitionVoie)
	}
	fmt.Fprintf(&sb, " %s %s, %s %s", pa.LibelleTypeDeVoie, pa.LibelleVoie, pa.CodePostal, pa.LibelleCommune)

	address := sb.String()
	// TODO: check if this uppper / lower stuff is what we really want
	columns := []string{
		pa.PPId,
		strings.ToLower(pa.Prenom),
		strings.ToLower(pa.Nom),
		address,
	}
	// s := pa.Rest
	return fmt.Fprintf(writer, "%s\n", strings.Join(columns, "|"))
}

func computeAllNGrams(pa *PersonActivityRecord, n int) []string {
	s := fmt.Sprintf("%s %s", pa.Name, pa.LastName)
	return ngrams(s, n)
}

func ngrams(s string, N int) []string {
	var ngrams []string

	sz := len(s)
	runeLen := utf8.RuneCountInString(s)
	if sz != runeLen {
		// TODO: do better than panic ?
		panic(fmt.Sprintf("sz (%d) != runeLen (%d) for `%v` \n", sz, runeLen, s))
	}

	for i, _ := range s {
		if (i + N) > sz {
			break
		}
		ngram := s[i : i+N]
		ngrams = append(ngrams, ngram)
	}

	return ngrams
}

func filterAndWriteData(reader *bufio.Reader, writer *bufio.Writer) error {
	_, headerErr := reader.ReadString('\n')
	if headerErr != nil {
		return headerErr
	}

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if line != "" {
				// TODO: process this semi-line ? or not... just discard it
			}
			break
		}
		if err != nil {
			return err
		}
		record, err := parseRawRecordFromLine(line)
		if err != nil {
			return err
		}
		if record.PPIdType != 8 || record.CodeProfession != "10" || record.CodeCategorieProfessionnelle == "M" || record.CodeModeExercice != "L" {
			// TODO: check that names/surnames do not have any special chars
			continue
		}

		// const NGramSize = 3
		// _ = computeAllNGrams(record, NGramSize)
		// TODO: put our n-grams into some superstructure (hash-table ?) that
		// has a mapping of n-gram -> [corresponding file offsets]

		_, err = writeRecord(record, writer)
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}
