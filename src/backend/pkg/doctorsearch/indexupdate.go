package doctorsearch

import (
	"archive/zip"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// An HTTP client that only trusts an "ASIP Sante" root certificate.
var httpClient *http.Client

func init() {
	// ASIP Sante root certificate.
	// See http://igc-sante.esante.gouv.fr/PC/
	const ASIPRootCertPEM = `
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

	rootCA := x509.NewCertPool()
	ok := rootCA.AppendCertsFromPEM([]byte(ASIPRootCertPEM))
	if !ok {
		log.Fatal().Msg("Error adding ASIP SANTE certificate to cert pool")
	}

	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).DialContext,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: 2 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs: rootCA,
		},
	}
	httpClient = &http.Client{Transport: tr}
}

type NextUpdate int

const (
	NormalUpdate NextUpdate = iota
	FastUpdate
)

var (
	errUnexpectedZIP = errors.New("ZIP file did not contain expected data file")
)

type indexUpdater struct {
	updatePeriod    time.Duration
	updateMinPeriod time.Duration
	// Jitter percentage, expressed between 0.0 and 1.0
	updatePeriodJitter float32
}

func (iu *indexUpdater) nextUpdateDelay(next NextUpdate) time.Duration {
	var p time.Duration
	if next == NormalUpdate {
		p = iu.updatePeriod
	} else if next == FastUpdate {
		p = iu.updateMinPeriod
	}

	jitterFactor := float64(iu.updatePeriodJitter) * (2*rand.Float64() - 1)
	jitter := time.Duration(jitterFactor * float64(p))
	return p + jitter
}

func (iu *indexUpdater) Start(firstUpdate NextUpdate, dr *drSearcher) {
	ticker := time.NewTicker(math.MaxInt64)
	nextUpdate := firstUpdate

	for {
		nextUpdateDelay := iu.nextUpdateDelay(nextUpdate)
		ticker.Reset(nextUpdateDelay)
		log.Trace().
			Dur("delay", nextUpdateDelay).
			Msg("scheduled next data file update")
		<-ticker.C

		tmpFilePath, cleanup, err := downloadNewData()
		if err != nil {
			log.Error().Msgf("error downloading new data file: %s", err)
			nextUpdate = FastUpdate
			cleanup()
			continue
		}

		index, err := buildIndex(tmpFilePath, dr.nGramSize)
		if err != nil {
			log.Error().Msgf("error building new index from new data: %s", err)
			nextUpdate = FastUpdate
		} else {
			dr.indexControl.UseIndex(index)
			nextUpdate = NormalUpdate

			// On succesful index creation and use, move the downloaded data file
			// to the canonical location, i.e. overwrite the previous records and replace
			// them with the
			moveErr := os.Rename(tmpFilePath, dr.dataFilePath)
			if moveErr != nil {
				log.Error().Msgf("error overwriting data file: %s", moveErr)
			}
		}

		cleanup()
	}
}

// downloadNewData checks for an update to the doctor data file.
// It returns a path to the new data file, which is completely owned by the caller:
// it can be moved, renamed etc...
// To avoid leaking resources (disk space), the caller should call the cleanup
// function as soon as they do not need this file anymore.
func downloadNewData() (string, func(), error) {
	const URL = "https://service.annuaire.sante.fr/annuaire-sante-webservices/V300/services/extraction/PS_LibreAcces"
	const FileNamePrefix = "PS_LibreAcces_Personne_activite_"

	resp, err := httpClient.Get(URL)
	if err != nil {
		return "", func() {}, err
	}
	log.Trace().
		Int64("http_content_length", resp.ContentLength).
		Msg("downloading data file")

	tmpDir, err := ioutil.TempDir("", "download-task-*")
	cleanupFunc := func() {
		os.RemoveAll(tmpDir)
	}
	if err != nil {
		return "", cleanupFunc, err
	}
	log.Trace().
		Str("tmp_dir_path", tmpDir).
		Msg("created temporary directory for download")

	tmpFile, err := ioutil.TempFile(tmpDir, "*.zip")
	if err != nil {
		return "", cleanupFunc, err
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", cleanupFunc, err
	}

	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		return "", cleanupFunc, err
	}
	fi, err := tmpFile.Stat()
	if err != nil {
		return "", cleanupFunc, err
	}

	zipr, err := zip.NewReader(tmpFile, fi.Size())
	if err != nil {
		return "", cleanupFunc, err
	}

	var zipFile *zip.File
	for _, f := range zipr.File {
		if strings.HasPrefix(f.Name, FileNamePrefix) {
			zipFile = f
			break
		}
	}
	if zipFile == nil {
		return "", cleanupFunc, errUnexpectedZIP
	}

	tmpDataFile, err := ioutil.TempFile(tmpDir, "data-*.txt")
	if err != nil {
		return "", cleanupFunc, err
	}
	defer tmpDataFile.Close()

	zipFileReader, err := zipFile.Open()
	if err != nil {
		return "", cleanupFunc, err
	}
	defer zipFileReader.Close()

	_, err = io.Copy(tmpDataFile, zipFileReader)
	if err != nil {
		return "", cleanupFunc, err
	}

	return tmpDataFile.Name(), cleanupFunc, nil
}
