package datamap

import (
	"fmt"
	"sync"
	"time"
	"strconv"
	"html/template"

	"autocontract/internal/uuid"

	"github.com/vincent-petithory/dataurl"
)

const (
	Docteur  string = "Docteur"
	Monsieur string = "Monsieur"
	Madame   string = "Madame"
)

type Person struct {
	Name                 string
	HonorificTitle       string
	NumberRPPS           string
	// Only applies to regular doctors
	NumberADELI          string
	// Only applies to substitute doctors
	NumberSubstitutingID string
	Address              string
	SignatureImgHtml     string
}

// TODO: figure out if this is worth it in the contract templates
func (p *Person) masculinOrFeminin(masculin string, feminin string) string {
	h := p.HonorificTitle
	if h == Madame {
		return feminin
	}
	return masculin
}

func (p *Person) Designation() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return fmt.Sprintf("Docteur %s", p.Name), nil
	} else if h == Monsieur {
		return fmt.Sprintf("Monsieur %s", p.Name), nil
	} else if h == Madame {
		return fmt.Sprintf("Madame %s", p.Name), nil
	}
	return "", fmt.Errorf("Unexpected Designation '%s'", h)
}

func (p *Person) ShortDesignation() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return fmt.Sprintf("Dr. %s", p.Name), nil
	} else if h == Monsieur {
		return fmt.Sprintf("M. %s", p.Name), nil
	} else if h == Madame {
		return fmt.Sprintf("Mme. %s", p.Name), nil
	}
	return "", fmt.Errorf("Unexpected ShortDesignation")
}

func (p *Person) OfficialCapacity() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return "un médecin inscrit au Tableau de l'Ordre", nil
	} else if (h == Monsieur || h == Madame)  {
		return "un étudiant en médecine titulaire d'une licence de remplacement", nil
	}
	return "", fmt.Errorf("Unexpected OfficialCapacity")
}

func (p *Person) ShortOfficialCapacity() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return "médecin remplaçant", nil
	} else if (h == Monsieur || h == Madame)  {
		return "étudiant en médecine", nil
	}
	return "", fmt.Errorf("Unexpected ShortOfficialCapacity")
}

func (p *Person) SubstitutingDescription() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return fmt.Sprintf("Numéro d'inscription au tableau: %s", p.NumberSubstitutingID), nil
	} else if (h == Monsieur || h == Madame)  {
		return fmt.Sprintf("Licence de remplacement N° %s", p.NumberSubstitutingID), nil
	}
	return "", fmt.Errorf("Unexpected SubstitutingDescription")
}

func (p *Person) SafeSignatureImgHtml() (template.HTML, error) {
	rawImgData := p.SignatureImgHtml

	var htmlStr string
	if (rawImgData == "") {
		htmlStr = `<img alt="" src="">`
	} else {
		htmlStr = fmt.Sprintf(`<img alt="" src="%s">`, rawImgData)
	}

	return template.HTML(htmlStr), nil
}

type Period struct {
	Start time.Time
	End   time.Time
}

type timeFormatted struct {
	year string
	month string
	day string
}

type periodFormatted struct {
	start timeFormatted
	end timeFormatted
}

var frenchMonths = [...]string{
	"Janvier",
	"Février",
	"Mars",
	"Avril",
	"Mai",
	"Juin",
	"Juillet",
	"Août",
	"Septembre",
	"Octobre",
	"Novembre",
	"Décembre",
}

func (p *Period) formatted() periodFormatted {
	forFirst := func(day int) string {
		if day == 1 {
			return "1er"
		}
		return strconv.Itoa(day)
	}

	return periodFormatted{
		start: timeFormatted{
			strconv.Itoa(p.Start.Year()),
			frenchMonths[p.Start.Month()-1],
			forFirst(p.Start.Day()),
		},
		end: timeFormatted{
			strconv.Itoa(p.End.Year()),
			frenchMonths[p.End.Month()-1],
			forFirst(p.End.Day()),
		},
	}
}

type GardesFinancials struct {
	Differs              bool
	HonorairesPercentage int
}

type Financials struct {
	HonorairesPercentage int
	Gardes               GardesFinancials
}

type UserData struct {
	Replaced                Person
	Substituting            Person
	Periods                 []Period
	Financials              Financials
	DateContractEstablished time.Time
}

func (p *Period) duration() (days int) {
	a := p.Start
	b := p.End

	if a.Location() != b.Location() {
		b = b.In(a.Location())
	}
	if a.After(b) {
		a, b = b, a
	}

	duration := b.Sub(a)
	hours := duration.Hours()

	days = int(hours / 24)
	// add 1 as the bounds are inclusive
	days += 1

	return
}

func (u *UserData) totalDuration() (days int) {
	days = 0
	for _, period := range u.Periods {
		diffDays := period.duration()
		days += diffDays
	}
	return
}

func (u *UserData) FormattedDuration() string {
	days := u.totalDuration()

	if days == 1 {
		return "1 jour"
	}
	return fmt.Sprintf("%d jours", days)
}

// TODO: fix & test this
func (u *UserData) FormattedPeriods() string {
	if len(u.Periods) == 0 {
		return ""
	}

	var formattedPeriods []periodFormatted

	areAllPeriodsInSameYear := true
	firstPeriodYear := u.Periods[0].formatted().start.year
	for _, period := range u.Periods {
		fp := period.formatted()
		formattedPeriods = append(formattedPeriods, fp)

		if fp.start.year != firstPeriodYear || fp.end.year != firstPeriodYear {
			areAllPeriodsInSameYear = false
		}
	}

	formatPeriod := func (pf periodFormatted, isLast bool) string {
		if pf.start == pf.end {
			s := fmt.Sprintf("le %s %s", pf.start.day, pf.start.month)
			if !areAllPeriodsInSameYear || isLast {
				s += fmt.Sprintf(" %s", pf.end.year)
			}
			return s
		}

		var s string
		if pf.start.month == pf.end.month && pf.start.year == pf.end.year {
			s = fmt.Sprintf("du %s au %s %s", pf.start.day, pf.end.day, pf.end.month)
		} else {
			startYear := ""
			if pf.start.year != pf.end.year {
				startYear = fmt.Sprintf(" %s", pf.start.year)
			}
			s = fmt.Sprintf("du %s %s%s au %s %s",
				pf.start.day, pf.start.month, startYear, pf.end.day, pf.end.month)
		}

		if !areAllPeriodsInSameYear || isLast {
			s += fmt.Sprintf(" %s", pf.end.year)
		}
		s += " compris"

		return s
	}

	s := ""
	for idx, fp := range formattedPeriods {
		numDays := u.Periods[idx].duration()
		isLastElement := idx == len(formattedPeriods) - 1

		if idx == 0 && numDays == 1 {
			s += "pour "
		} else if idx > 0 && !isLastElement {
			s += ", "
		} else if len(formattedPeriods) > 1 && isLastElement {
			s += " et "
		}

		s += fmt.Sprintf("%s", formatPeriod(fp, isLastElement))
	}
	return s
}

// TODO: test this
func (u *UserData) FormattedDateContractEstablished() string {
	const timeLayout = "02/01/2006"
	return u.DateContractEstablished.Format(timeLayout)
}

type SafeUserData interface {
	GetUserData() UserData
}

type safeUserData struct {
	userData UserData
}

func (s *safeUserData) GetUserData() UserData {
	return s.userData
}

func SanitizeUserData(u *UserData) (SafeUserData, error) {
	err := sanitizePeriods(u.Periods)
	if err != nil {
		return nil, err
	}
	err = sanitizePerson(&u.Replaced)
	if err != nil {
		return nil, err
	}
	err = sanitizePerson(&u.Substituting)
	if err != nil {
		return nil, err
	}
	return &safeUserData{
		userData: *u,
	}, nil
}

func sanitizePerson(p *Person) error {
	nameLength := len([]rune(p.Name))
	if nameLength > 200 {
		return fmt.Errorf("maximum length of name is 200 characters (input is %d)", nameLength)
	}
	_, err := p.Designation()
	if err != nil {
		return err
	}

	rppsLength := len(p.NumberRPPS)
	if rppsLength < 11 || rppsLength > 50 {
		return fmt.Errorf("invalid RPPS length (%d)", rppsLength)
	}

	adeliLength := len(p.NumberADELI)
	// Only check ADELI number if it is specified
	if adeliLength > 50 {
		return fmt.Errorf("invalid ADELI length (%d)", adeliLength)
	}

	substitutingIDLength := len(p.NumberSubstitutingID)
	// Only check "SubstitutingID" number if it is specified
	if substitutingIDLength > 50 {
		return fmt.Errorf("invalid NumberSubstitutingID length (%d)", substitutingIDLength)
	}

	addressLength := len(p.Address)
	if addressLength > 400 {
		return fmt.Errorf("maximum size of address is 400 bytes (input is %d)", addressLength)
	}

	signatureSize := len(p.SignatureImgHtml)
	if (signatureSize == 0) {
		return nil
	}
	fmt.Printf("!!!!! Signature size is %dkB\n", signatureSize / 1024)
	if (signatureSize > 300 * 1024) {
		p.SignatureImgHtml = ""
		return fmt.Errorf("maximum size of base64 encoded signature image is 300kB (input is %dkB)", signatureSize / 1024)
	}
	dataURL, signatureErr := dataurl.DecodeString(p.SignatureImgHtml)
	if (signatureErr != nil) {
		p.SignatureImgHtml = ""
		return signatureErr
	}

	if dataURL.ContentType() != "image/svg+xml" {
		p.SignatureImgHtml = ""
		return fmt.Errorf("unexpected media type in signature dataurl '%s'", dataURL.ContentType())
	}
	// Encode the data ourselves to be sure the Data URL is exactly what we expect
	// and we can stuff the value into the "src" attribute of an HTML <img> element.
	safeDataURL := &dataurl.DataURL{
		MediaType: dataurl.MediaType{
			"image",
			"svg+xml",
			map[string]string{},
		},
		Encoding: dataurl.EncodingBase64,
		Data: dataURL.Data,
	}
	p.SignatureImgHtml = safeDataURL.String()

	return nil
}

func sanitizePeriods(periods []Period) error {
	const MaxPeriods = 50
	if len(periods) < 1 {
		return fmt.Errorf("need at least one period")
	} else if len(periods) > MaxPeriods {
		return fmt.Errorf("maximum number of periods is %d (input contains %d)", MaxPeriods, len(periods))
	}
	return nil
}

type DataMap interface {
	Get(key string) (*UserData, error)
	Set(data SafeUserData) (string, error)
	Clear(key string)
}

type dataMap struct {
	internalMap sync.Map
}

func NewDataMap() DataMap {
	return &dataMap{
		internalMap: sync.Map{},
	}
}

func (m *dataMap) Get(key string) (*UserData, error) {
	value, ok := m.internalMap.Load(key)
	if !ok {
		return nil, fmt.Errorf("no user data found for key '%s'", key)
	}
	u := value.(UserData)
	return &u, nil
}

func (m *dataMap) Set(data SafeUserData) (string, error) {
	uuid, err := uuid.NewUuid()
	if err != nil {
		return "", err
	}
	key := uuid.String()
	m.internalMap.Store(key, data.GetUserData())
	return key, nil
}

func (m *dataMap) Clear(key string) {
	m.internalMap.Delete(key)
}
