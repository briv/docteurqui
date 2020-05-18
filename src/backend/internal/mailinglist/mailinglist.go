package mailinglist

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"

	"github.com/rs/zerolog/log"
	"github.com/sasha-s/go-csync"
)

var (
	InvalidEmail = errors.New("invalid email")

	MaxEmailByteLength = 300
)

// GenerateKeyFiles creates two key files, one for the public key, one for the private key.
//
// If the specified directory is empty, it will create a temporary directory.
func GenerateKeyFiles(parentDirectory string) (pubKeyPath string, privateKeyPath string, err error) {
	dir, err := ioutil.TempDir(parentDirectory, "*-keyfiles")
	if err != nil {
		return
	}

	pubKeyA, privKeyA, err := box.GenerateKey(cryptorand.Reader)
	if err != nil {
		return
	}

	pubKey := pubKeyA[:]
	privKey := privKeyA[:]

	p := make([]byte, base64.StdEncoding.EncodedLen(len(pubKey)))
	base64.StdEncoding.Encode(p, pubKey)
	tmpPubKeyPath := path.Join(dir, "public.key")
	err = ioutil.WriteFile(tmpPubKeyPath, p, 0644)
	if err != nil {
		return
	}

	p = make([]byte, base64.StdEncoding.EncodedLen(len(privKey)))
	base64.StdEncoding.Encode(p, privKey)
	tmpPrivateKeyPath := path.Join(dir, "private.key.secret")
	err = ioutil.WriteFile(tmpPrivateKeyPath, p, 0600)
	if err != nil {
		return
	}

	pubKeyPath = tmpPubKeyPath
	privateKeyPath = tmpPrivateKeyPath

	return
}

func readKeyFromFile(filePath string) (*[32]byte, error) {
	rawBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	maxN := base64.StdEncoding.DecodedLen(len(rawBytes))
	dst := make([]byte, maxN)

	n, err := base64.StdEncoding.Decode(dst, rawBytes)
	if err != nil {
		return nil, err
	}
	if n != 32 {
		return nil, fmt.Errorf("incorrect key length %d bytes", n)
	}

	var key [32]byte
	copy(key[:], dst[:32])
	return &key, nil
}

type MailingLister interface {
	Add(context.Context, string) error
}

type mailingLister struct {
	mailingListFile io.Writer
	pubKey          *[32]byte
	mutex           csync.Mutex
}

// New returns a MailingLister.
//
// mailingListFilePath should be the
// pubKeyBase64 should be a file containing an NaCl box public key, encoded with base64.
func New(mailingListFilePath string, publicKeyPath string) (MailingLister, error) {
	publicKey, err := readKeyFromFile(publicKeyPath)
	if err != nil {
		return nil, err
	}

	flag := os.O_WRONLY | os.O_APPEND | os.O_CREATE
	mailingListFile, err := os.OpenFile(path.Clean(mailingListFilePath), flag, 0600)
	if err != nil {
		return nil, fmt.Errorf("issue with mailing list file")
	}

	m := &mailingLister{
		pubKey:          publicKey,
		mailingListFile: mailingListFile,
	}
	return m, nil
}

func (m *mailingLister) Add(ctx context.Context, rawEmail string) error {
	if len(rawEmail) > MaxEmailByteLength {
		return InvalidEmail
	}
	email := strings.TrimSpace(rawEmail)

	if !strings.ContainsRune(email, '@') || !strings.ContainsRune(email, '.') || strings.ContainsRune(email, '\t') {
		return InvalidEmail
	}

	if err := m.mutex.CLock(ctx); err != nil {
		// Failed to lock.
		return err
	}
	defer m.mutex.Unlock()

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s\t%s", time.Now().UTC().Format(time.RFC3339), email)
	box, err := box.SealAnonymous(nil, buf.Bytes(), m.pubKey, nil)
	if err != nil {
		return err
	}

	dataToWrite := make([]byte, 1+base64.StdEncoding.EncodedLen(len(box)))
	dataToWrite[0] = '\n'
	base64.StdEncoding.Encode(dataToWrite[1:], box)

	if _, err := m.mailingListFile.Write(dataToWrite); err != nil {
		// This is pretty bad but not much we can do about it.
		return err
	}

	return nil
}

type MailingListReader interface {
	ReadAll() ([]string, error)
}

type reader struct {
	file       io.ReadSeeker
	privateKey *[32]byte
	publicKey  *[32]byte
}

func NewReader(mailingListFilePath string, privateKeyPath string) (MailingListReader, error) {
	key, err := readKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	flag := os.O_RDONLY
	mailingListFile, err := os.OpenFile(path.Clean(mailingListFilePath), flag, 0)

	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, key)

	r := &reader{
		file:       mailingListFile,
		privateKey: key,
		publicKey:  &pubKey,
	}

	return r, nil
}

func (r *reader) ReadAll() ([]string, error) {
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var bEmails [][]byte

	scanner := bufio.NewScanner(r.file)
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(b) > 0 {
			p := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
			if n, err := base64.StdEncoding.Decode(p, b); err == nil {
				bEmails = append(bEmails, p[:n])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	log.Trace().
		Str("pub_key", base64.StdEncoding.EncodeToString(r.publicKey[:])).
		Msg("reading mailinglist")

	var emails []string

	for _, bEmail := range bEmails {
		message, ok := box.OpenAnonymous(nil, bEmail, r.publicKey, r.privateKey)
		if ok {
			emails = append(emails, string(message))
		}
	}

	log.Trace().
		Int("num_emails", len(emails)).
		Int("num_errors", len(bEmails)-len(emails)).
		Int("num_boxes", len(bEmails)).
		Msg("done reading mailinglist")

	return emails, nil
}
