package form

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"autocontract/internal/datamap"
	"autocontract/internal/validation"

	"github.com/rs/zerolog/log"
	"github.com/vincent-petithory/dataurl"
)

const (
	MaxNameLength    = 200
	RPPSLength       = 11
	SIRETLength      = 14
	MaxTitleLength   = 50
	MaxGenericLength = 400

	MaxSignatureSizeBytes = 300 * 1024
)

type FormProcessingManner struct {
	TimeLayout   string
	TimeLocation *time.Location
}

type validationFunc func(string) (string, error)

func requiredField(value string) (string, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return "", validation.MissingRequired
	}
	return s, nil
}

func oneOf(values []string) validationFunc {
	return func(value string) (s string, err error) {
		for _, whitelistedValue := range values {
			if value == whitelistedValue {
				return value, err
			}
		}
		return "", fmt.Errorf("%w '%s'", validation.ParseError, value)
	}
}

func minLength(length int) validationFunc {
	return func(value string) (s string, err error) {
		rs := []rune(value)
		if len(rs) < length {
			return "", fmt.Errorf("%w (%d), under limit (%d)", validation.LengthError, len(rs), length)
		}
		return value, nil
	}
}

func maxLength(length int) validationFunc {
	return func(value string) (s string, err error) {
		rs := []rune(value)
		if len(rs) > length {
			return "", fmt.Errorf("%w (%d) is over limit (%d)", validation.LengthError, len(rs), length)
		}
		return value, nil
	}
}

func validateField(name string, r *http.Request, issues validation.ValidationIssues, validators []validationFunc) string {
	var value string = r.PostFormValue(name)
	var err error

	for _, validationF := range validators {
		value, err = validationF(value)
		if err != nil {
			issues.Set(name, err)
			break
		}
	}
	return value
}

var (
	NameValidators = []validationFunc{
		requiredField, maxLength(MaxNameLength),
	}
	RPPSValidators = []validationFunc{
		requiredField, maxLength(RPPSLength), minLength(RPPSLength),
	}
	SIRETValidators = []validationFunc{
		requiredField, maxLength(SIRETLength), minLength(SIRETLength),
	}
	TitleValidators = []validationFunc{
		requiredField, maxLength(MaxTitleLength), oneOf([]string{datamap.Docteur, datamap.Madame, datamap.Monsieur}),
	}
	GenericMaxLength = []validationFunc{
		requiredField, maxLength(MaxGenericLength),
	}
)

func sanitizeSignature(rawData string) (string, error) {
	signatureSize := len(rawData)
	if signatureSize == 0 {
		return "", nil
	}

	if signatureSize > MaxSignatureSizeBytes {
		return "", fmt.Errorf("%w, maximum size of base64 encoded signature image is %dkB (input is %dkB)", validation.LengthError, MaxSignatureSizeBytes/1024, signatureSize/1024)
	}
	dataURL, signatureErr := dataurl.DecodeString(rawData)
	if signatureErr != nil {
		return "", fmt.Errorf("%w, %s", validation.ParseError, signatureErr)
	}

	if dataURL.ContentType() != "image/svg+xml" {
		return "", fmt.Errorf("%w, unexpected media type in signature dataurl '%s'", validation.ParseError, dataURL.ContentType())
	}
	// Encode the data ourselves to be sure the Data URL is exactly what we expect
	// and we can stuff the value into the "src" attribute of an HTML <img> element.
	safeDataURL := &dataurl.DataURL{
		MediaType: dataurl.MediaType{
			Type:    "image",
			Subtype: "svg+xml",
			Params:  map[string]string{},
		},
		Encoding: dataurl.EncodingBase64,
		Data:     dataURL.Data,
	}
	return safeDataURL.String(), nil
}

func sanitizePeriods(periods []datamap.Period) ([]datamap.Period, validation.ValidationIssues) {
	const MaxPeriods = 30
	issues := validation.EmptyIssues()
	if len(periods) > MaxPeriods {
		// return fmt.Errorf("maximum number of periods is %d (input contains %d)", MaxPeriods, len(periods))
		issues.Set("period-start", validation.TooMany)
		issues.Set("period-end", validation.TooMany)
	}
	return periods, issues
}

func Process(r *http.Request, manner FormProcessingManner) (datamap.SafeUserData, error) {
	validationIssues := validation.EmptyIssues()

	var periods []datamap.Period
	periodStartsStr := r.PostForm["period-start"]
	periodEndsStr := r.PostForm["period-end"]
	if len(periodStartsStr) == 0 {
		validationIssues.Set("period-start", validation.MissingRequired)
	}
	if len(periodEndsStr) == 0 {
		validationIssues.Set("period-end", validation.MissingRequired)
	}

	for index, periodStartStr := range periodStartsStr {
		periodStart, err := time.Parse(manner.TimeLayout, periodStartStr)
		if err != nil {
			validationIssues.Set("period-start", validation.ParseError)
			break
		}

		if index >= len(periodEndsStr) {
			log.Trace().Msgf("invalid periods (%d starts, %d ends)", len(periodStartsStr), len(periodEndsStr))
			validationIssues.Set("period-start", validation.TooMany)
			break
		}

		periodEndStr := periodEndsStr[index]
		periodEnd, err := time.Parse(manner.TimeLayout, periodEndStr)
		if err != nil {
			validationIssues.Set("period-end", validation.ParseError)
			break
		}

		periods = append(periods, datamap.Period{
			Start: periodStart.In(manner.TimeLocation),
			End:   periodEnd.In(manner.TimeLocation),
		})
	}
	periods, issues := sanitizePeriods(periods)
	validationIssues.Merge(issues)

	regularName := validateField("regular-name", r, validationIssues, NameValidators)
	regularRPPS := validateField("regular-rpps", r, validationIssues, RPPSValidators)

	safeRegularSignature, err := sanitizeSignature(r.PostFormValue("regular-signature"))
	if err != nil {
		validationIssues.Set("regular-signature", err)
	}

	regularDoctor := datamap.Person{
		Name:             regularName,
		HonorificTitle:   datamap.Docteur,
		NumberRPPS:       regularRPPS,
		Address:          validateField("regular-address", r, validationIssues, GenericMaxLength),
		SignatureImgHtml: safeRegularSignature,
	}

	substituteName := validateField("substitute-name", r, validationIssues, NameValidators)
	substituteTitle := validateField("substitute-title", r, validationIssues, TitleValidators)
	substituteRPPS := validateField("substitute-rpps", r, validationIssues, RPPSValidators)
	substituteSIRET := validateField("substitute-siret", r, validationIssues, SIRETValidators)

	safeSubstituteSignature, err := sanitizeSignature(r.PostFormValue("substitute-signature"))
	if err != nil {
		validationIssues.Set("substitute-signature", err)
	}

	substituting := datamap.Person{
		Name:                 substituteName,
		HonorificTitle:       substituteTitle,
		NumberRPPS:           substituteRPPS,
		NumberSIRET:          substituteSIRET,
		NumberSubstitutingID: validateField("substitute-substitutingID", r, validationIssues, GenericMaxLength),
		Address:              validateField("substitute-address", r, validationIssues, GenericMaxLength),
		SignatureImgHtml:     safeSubstituteSignature,
	}

	retrocessionPercStr := validateField("financials-retrocession", r, validationIssues, []validationFunc{
		requiredField,
	})
	retrocessionPerc, err := strconv.Atoi(retrocessionPercStr)
	if err != nil {
		validationIssues.Set("financials-retrocession", validation.ParseError)
	}

	nightShiftRetrocessionPerc := retrocessionPerc
	nightShiftRetrocessionPercStr := strings.TrimSpace(r.PostFormValue("financials-nightShiftRetrocession"))
	if nightShiftRetrocessionPercStr != "" {
		nightShiftRetrocessionPerc, err = strconv.Atoi(nightShiftRetrocessionPercStr)
		if err != nil {
			validationIssues.Set("financials-nightShiftRetrocession", validation.ParseError)
		}
	}

	retrocessionsDiffer := retrocessionPerc != nightShiftRetrocessionPerc
	financials := datamap.Financials{
		HonorairesPercentage: retrocessionPerc,
		Gardes: datamap.GardesFinancials{
			Differs:              retrocessionsDiffer,
			HonorairesPercentage: nightShiftRetrocessionPerc,
		},
	}

	if err := validationIssues.Error(); err != nil {
		return nil, err
	}

	return datamap.MarkSafe(datamap.UserData{
		Regular:                 regularDoctor,
		Substituting:            substituting,
		Periods:                 periods,
		Financials:              financials,
		DateContractEstablished: time.Now().In(manner.TimeLocation),
	}), nil
}
