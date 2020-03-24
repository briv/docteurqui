import TinyDatePicker from 'tiny-date-picker';
import { makeElement } from './utils';

const FrenchMonths = [
    'Janvier',
    'Février',
    'Mars',
    'Avril',
    'Mai',
    'Juin',
    'Juillet',
    'Août',
    'Septembre',
    'Octobre',
    'Novembre',
    'Décembre',
];

export const parseFormattedFRDate = (str) => {
    let date;
    if (typeof str === 'string') {
        const [dayStr, monthStr, yearStr] = str.split(' ');
        const year = parseInt(yearStr, 10);
        const month = FrenchMonths.findIndex(el => el == monthStr);
        const day = parseInt(dayStr, 10);
        date = new Date(year, month, day);
    } else {
        date = new Date(str);
    }

    if (isNaN(date)) {
        throw new Error(`invalid date ${date}`);
    }
    return date;
}

const dayPlusOne = (date) => {
    const result = new Date(date);
    result.setDate(result.getDate() + 1);
    return result;
};

const useCustomDatePicker = (() => {
    const cachedResult = true;

    return () => cachedResult;
})();

export const createSinglePeriodInput = (container, beforeSibling) => {
    const periodClass = 'period';
    const numberCurrentPeriodInputs = container.querySelectorAll(`.${periodClass}`).length;

    const rootContainer = makeElement('div', el => {
        el.classList.add('d-flex', 'flex-column', `${periodClass}`);
    })
    const dateRangeContainer = makeElement('div', el => {
        el.classList.add('d-flex', 'flex-row');
    })
    const labelContainer = makeElement('div', el => {
        el.classList.add('d-flex', 'flex-row');
    })

    const startLabel = makeElement('label', el => {
        if (numberCurrentPeriodInputs === 0) {
            el.textContent = 'Du';
        } else {
            el.textContent = 'et du';
        }
    });
    const endLabel = makeElement('label', el => {
        el.textContent = 'au';
    });

    const inputType = useCustomDatePicker() ? 'text' : 'date';
    const dateRangeStart = makeElement('input', el => {
        el.setAttribute('type', inputType);
        el.setAttribute('name', 'period-start');
    });
    const dateRangeEnd = makeElement('input', el => {
        el.setAttribute('type', inputType);
        el.setAttribute('name', 'period-end');
    });
    const removeIcon = makeElement('button', el => {
        el.classList.add('small', 'remove-period');
        el.textContent = '❌';
        el.setAttribute('type', 'button');

        if (numberCurrentPeriodInputs == 0) {
            el.setAttribute('disabled', true);
            el.setAttribute('aria-hidden', true);
            el.classList.add('hidden');
        }
    });

    for (let el of [startLabel, endLabel, removeIcon]) {
        labelContainer.appendChild(el);
    }
    for (let el of [dateRangeStart, dateRangeEnd]) {
        dateRangeContainer.appendChild(el);
    }
    for (let el of [labelContainer, dateRangeContainer]) {
        rootContainer.appendChild(el);
    }

    removeIcon.addEventListener('click', event => {
        event.preventDefault();
        rootContainer.remove();
    })

    container.insertBefore(rootContainer, beforeSibling);

    const options = {
        mode: 'dp-modal',
        lang: {
            days: ['Dim', 'Lun', 'Mar', 'Mer', 'Jeu', 'Ven', 'Sam'],
            months: [...FrenchMonths],
            today: "Aujourd'hui",
            clear: 'Effacer',
            close: 'Fermer',
        },
        format(date) {
            const day = date.getDate();
            const month = FrenchMonths[date.getMonth()];
            const year = date.getFullYear();
            return `${day} ${month} ${year}`;
        },
        parse(str) {
            try {
                return parseFormattedFRDate(str);
            } catch (e) {
                return new Date();
            }
        },
        dayOffset: 1,
    };

    if (useCustomDatePicker()) {
        const inRangeStart = (dt) => {
            if (dateRangeEnd.value === '') {
                return true;
            }

            const otherDate = parseFormattedFRDate(dateRangeEnd.value);
            if (dt > otherDate) {
                return false;
            }
            return true;
        };

        const inRangeEnd = (dt) => {
            if (dateRangeStart.value === '') {
                return true;
            }

            const otherDate = parseFormattedFRDate(dateRangeStart.value);
            if (dt < otherDate) {
                return false;
            }
            return true;
        };
        TinyDatePicker(dateRangeStart, { ...options, inRange: inRangeStart });
        const tinyDatePickerEnd = TinyDatePicker(dateRangeEnd, { ...options, inRange: inRangeEnd });

        tinyDatePickerEnd.on('open', (_, dp) => {
            // Do not do anything if user has already selected end date
            // or if no start date is selected
            if (dateRangeEnd.value !== ''
                || dateRangeStart.value === '') {
                return;
            }
            const startSelectedDate = parseFormattedFRDate(dateRangeStart.value);

            // Otherwise, highlight the start day + 1
            dp.setState({
                hilightedDate: dayPlusOne(startSelectedDate),
            });
        });
    }
}
