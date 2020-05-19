package datamap

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"sync"
	"time"

	"autocontract/pkg/uuid"
)

const (
	Docteur  string = "Docteur"
	Monsieur string = "Monsieur"
	Madame   string = "Madame"
)

type Person struct {
	Name           string
	HonorificTitle string
	NumberRPPS     string
	// Only applies to substitute doctors
	NumberSIRET          string
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
	} else if h == Monsieur || h == Madame {
		return "un étudiant en médecine titulaire d'une licence de remplacement", nil
	}
	return "", fmt.Errorf("Unexpected OfficialCapacity")
}

func (p *Person) ShortOfficialCapacity() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return "médecin remplaçant", nil
	} else if h == Monsieur || h == Madame {
		return "étudiant en médecine", nil
	}
	return "", fmt.Errorf("Unexpected ShortOfficialCapacity")
}

func (p *Person) SubstitutingDescription() (string, error) {
	h := p.HonorificTitle
	if h == Docteur {
		return fmt.Sprintf("Numéro d'inscription au tableau: %s", p.NumberSubstitutingID), nil
	} else if h == Monsieur || h == Madame {
		return fmt.Sprintf("Licence de remplacement N° %s", p.NumberSubstitutingID), nil
	}
	return "", fmt.Errorf("Unexpected SubstitutingDescription")
}

func (p *Person) SafeSignatureImgHtml() (template.HTML, error) {
	rawImgData := p.SignatureImgHtml

	var htmlStr string
	if rawImgData == "" {
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
	year  string
	month string
	day   string
}

type periodFormatted struct {
	start timeFormatted
	end   timeFormatted
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
	Regular                 Person
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

	formatPeriod := func(pf periodFormatted, isLast bool) string {
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
		isLastElement := idx == len(formattedPeriods)-1

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
	const frenchDateLayout = "02/01/2006"
	return u.DateContractEstablished.Format(frenchDateLayout)
}

type SafeUserData interface {
	GetUserData() UserData
	// Identifier returns an string with personally identifiable information
	//  that can be used to check if contracts are essentially the same.
	//
	// Currently, this string is just the concatenation: regular RPPS | substitute RPPS | contract dates.
	Identifier() string
}

type safeUserData struct {
	userData UserData
}

func (s *safeUserData) GetUserData() UserData {
	return s.userData
}

func (s *safeUserData) Identifier() string {
	const timeAsDateLayout = "2006-01-02"
	const seperator = '|'

	u := s.userData
	var sb strings.Builder
	sb.WriteString(u.Regular.NumberRPPS)
	sb.WriteRune(seperator)
	sb.WriteString(u.Substituting.NumberRPPS)
	for _, p := range u.Periods {
		sb.WriteRune(seperator)
		sb.WriteString(p.Start.Format(timeAsDateLayout))
		sb.WriteRune(seperator)
		sb.WriteString(p.End.Format(timeAsDateLayout))
	}
	return sb.String()
}

func MarkSafe(u UserData) SafeUserData {
	return &safeUserData{
		userData: u,
	}
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
