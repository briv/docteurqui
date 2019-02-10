package datamap

import (
	"testing"
	"time"
)

type expectedValues struct {
	formattedDuration string
	formattedPeriods  string
}

func TestPeriods(t *testing.T) {
	tables := []struct {
		title    string
		periods  []Period
		expected expectedValues
	}{
		{
			"basic 2 days inclusive",
			[]Period{
				{
					time.Date(2019, time.February, 27, 0, 0, 0, 0, time.UTC),
					time.Date(2019, time.February, 28, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{
				"2 jours", "du 27 au 28 Février 2019 compris",
			},
		},
		{
			"basic months inclusive",
			[]Period{
				{
					time.Date(2019, time.April, 15, 0, 0, 0, 0, time.UTC),
					time.Date(2019, time.August, 15, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"123 jours", "du 15 Avril au 15 Août 2019 compris"},
		},
		{
			"months inclusive",
			[]Period{
				{
					time.Date(2019, time.April, 15, 0, 0, 0, 0, time.UTC),
					time.Date(2019, time.August, 17, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"125 jours", "du 15 Avril au 17 Août 2019 compris"},
		},
		{
			"bisextile year with february",
			[]Period{
				{
					time.Date(2016, time.February, 27, 0, 0, 0, 0, time.UTC),
					time.Date(2016, time.March, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"4 jours", "du 27 Février au 1er Mars 2016 compris"},
		},
		{
			"single day",
			[]Period{
				{
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"1 jour", "pour le 7 Juin 2018"},
        },
        {
			"2 single days",
			[]Period{
				{
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
                },
                {
					time.Date(2018, time.June, 9, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 9, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"2 jours", "pour le 7 Juin et le 9 Juin 2018"},
        },
        {
			"single days interspersed with period",
			[]Period{
				{
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 7, 0, 0, 0, 0, time.UTC),
                },
                {
					time.Date(2018, time.June, 9, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 11, 0, 0, 0, 0, time.UTC),
				},
                {
					time.Date(2018, time.June, 13, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 13, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"5 jours", "pour le 7 Juin, du 9 au 11 Juin compris et le 13 Juin 2018"},
		},
		{
			"multiple periods",
			[]Period{
				{
					time.Date(2018, time.June, 10, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 14, 0, 0, 0, 0, time.UTC),
                },
                {
					time.Date(2018, time.June, 16, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 16, 0, 0, 0, 0, time.UTC),
				},
				{
					time.Date(2018, time.June, 18, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 19, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"8 jours", "du 10 au 14 Juin compris, le 16 Juin et du 18 au 19 Juin 2018 compris"},
		},
		{
			"multiple periods across months and years",
			[]Period{
				{
					time.Date(2018, time.February, 8, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.February, 9, 0, 0, 0, 0, time.UTC),
				},
				{
					time.Date(2018, time.February, 27, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.March, 1, 0, 0, 0, 0, time.UTC),
                },
                {
					time.Date(2018, time.July, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.July, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					time.Date(2018, time.December, 29, 0, 0, 0, 0, time.UTC),
					time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"10 jours", "du 8 au 9 Février 2018 compris, du 27 Février au 1er Mars 2018 compris, le 1er Juillet 2018 et du 29 Décembre 2018 au 1er Janvier 2019 compris"},
		},
		{
			"entire calendar month",
			[]Period{
				{
					time.Date(2018, time.June, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.June, 30, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"30 jours", "du 1er au 30 Juin 2018 compris"},
		},
		{
			"from one day to the same day next month",
			[]Period{
				{
					time.Date(2018, time.June, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.July, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"31 jours", "du 1er Juin au 1er Juillet 2018 compris"},
		},
		{
			"across year boundary",
			[]Period{
				{
					time.Date(2019, time.December, 27, 0, 0, 0, 0, time.UTC),
					time.Date(2020, time.February, 26, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"62 jours", "du 27 Décembre 2019 au 26 Février 2020 compris"},
		},
		{
			"across year boundary",
			[]Period{
				{
					time.Date(2019, time.December, 27, 0, 0, 0, 0, time.UTC),
					time.Date(2020, time.February, 28, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"64 jours", "du 27 Décembre 2019 au 28 Février 2020 compris"},
		},
		{
			"across year boundary with day/month complication",
			[]Period{
				{
					time.Date(2019, time.December, 30, 0, 0, 0, 0, time.UTC),
					time.Date(2020, time.February, 27, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"60 jours", "du 30 Décembre 2019 au 27 Février 2020 compris"},
		},
		{
			"simple month + day count",
			[]Period{
				{
					time.Date(2017, time.February, 12, 0, 0, 0, 0, time.UTC),
					time.Date(2017, time.March, 27, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"44 jours", "du 12 Février au 27 Mars 2017 compris"},
        },
        {
			"same day, a year apart",
			[]Period{
				{
					time.Date(2017, time.February, 12, 0, 0, 0, 0, time.UTC),
					time.Date(2018, time.February, 12, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"366 jours", "du 12 Février 2017 au 12 Février 2018 compris"},
		},
		{
			"simple year count",
			[]Period{
				{
					time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
					time.Date(2017, time.March, 27, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"1854 jours", "du 29 Février 2012 au 27 Mars 2017 compris"},
		},
		{
			"year and month count with bisextile issue",
			[]Period{
				{
					time.Date(2012, time.February, 28, 0, 0, 0, 0, time.UTC),
					time.Date(2014, time.February, 28, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"732 jours", "du 28 Février 2012 au 28 Février 2014 compris"},
		},
		{
			"difficult year and month count",
			[]Period{
				{
					time.Date(2012, time.February, 29, 0, 0, 0, 0, time.UTC),
					time.Date(2015, time.February, 27, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedValues{"1095 jours", "du 29 Février 2012 au 27 Février 2015 compris"},
		},
	}

	for _, table := range tables {
		userData := UserData{
			Periods: table.periods,
		}

		// test duration formatting
		gotDuration := userData.FormattedDuration()
		wantDuration := table.expected.formattedDuration
		if gotDuration != wantDuration {
			t.Errorf("'%s', FormattedDuration() = \"%s\", expected \"%s\"", table.title, gotDuration, wantDuration)
		}

		// test periods formatting
		gotPeriods := userData.FormattedPeriods()
		wantPeriods := table.expected.formattedPeriods
		if gotPeriods != wantPeriods {
			t.Errorf("'%s', FormattedPeriods() = \"%s\", expected \"%s\"", table.title, gotPeriods, wantPeriods)
		}
	}
}
