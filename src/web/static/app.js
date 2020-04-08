import { v4 as uuidv4 } from 'uuid';

import { makeElement, GenericUserError, FormValidationError } from './utils';
import { createSignatureInput, getSignatureImage } from './signature';
import { saveFilledFormData, createPersistedDataQuickFillUI } from './form-fill';
import { Validators, ErrorHandler, FormValidationIssues } from './live-form-feedback';
import { createSinglePeriodInput, parseFormattedFRDate } from './periods-input';
import { polyfill } from './polyfills';
import { InputAutocompleter } from './autocomplete';

const ElementQueries = {
    SubstituteSignatureParent: 'fieldset#substitute-fieldset',
    RegularSignatureParent: 'fieldset#regular-fieldset',
};

const Globals = {
    SignaturePads: [],
    PreviousDataUI: [
        {

        }
    ],
};

const createDefaultPeriodInputUI = (form) => {
    const fieldsetElement = form.querySelector('.date-range-input');

    const rootDiv = makeElement('div', el => {
        el.classList.add('single-form-input-group');
    });

    const addDateInputButton = makeElement('button', el => {
        el.classList.add('small');
        el.setAttribute('type', 'button');
        el.textContent = 'Ajouter une autre pèriode';
    });
    rootDiv.appendChild(addDateInputButton);
    fieldsetElement.appendChild(rootDiv);

    createSinglePeriodInput(rootDiv, addDateInputButton);

    addDateInputButton.addEventListener('click', event => {
        event.preventDefault();
        createSinglePeriodInput(rootDiv, addDateInputButton);
    });
};

const setupFormIntercept = (form, UI) => {
    form.addEventListener('submit', (e) => {
        if (e.preventDefault) {
            e.preventDefault();
        }

        const rawFormData = new FormData(form);
        const url = form.action;
        const formData = processFormData(rawFormData);

        const submission = submitFormData(formData, url)
            .then(async (response) => {
                if (response.ok) {
                    return response;
                } else {
                    if (response.status == 422) {
                        const body = await response.json();
                        throw new FormValidationError(body);
                    }

                    const body = await response.text();
                    if (response.status >= 500) {
                        throw new GenericUserError(
                            "Une erreur que nous n'avions pas prévu s'est produite de notre côté, désolé !",
                            `submitting form: status=${response.status} body=${body}`
                        );
                    }

                    throw new GenericUserError(
                        "Une erreur vraiment inattendue s'est produite, désolé !",
                        `submitting form: status=${response.status} body=${body}`
                    );
                }
            })
            .then(response => response.blob())
            .then(blob => {
                const blobUrl = URL.createObjectURL(blob);
                var a = document.createElement('a');
                a.href = blobUrl;
                a.download = "Contrat remplacement.pdf";
                // We need to append the element to the dom, otherwise it will not work in firefox.
                document.body.appendChild(a);
                a.click();
                // Afterwards we remove the element again.
                a.remove();
                URL.revokeObjectURL(blobUrl);
            });

        submission.catch(err => UI.errorHandler(err));

        submission.then(() => {
            // If submission succeeds, serialize form data and save it locally.
            saveFilledFormData(formData);
        }).catch(() => {
            // Otherwise, do nothing. This prevents triggering the 'unhandledrejection' event.
        });

        return false;
    })
};

const processFormData = (data) => {
    // Special processing for dates (corresponding to "period-start" and "period-end" inputs).
    const processDates = (key) => data.getAll(key)
        .map(el => {
            try {
                const date = parseFormattedFRDate(el);
                return `${date.getFullYear()}-${(date.getMonth() +1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
            } catch (e) {
                return '';
            }
        });

    const DateStarts = processDates('period-start');
    const DateEnds = processDates('period-end');
    const DatePairs = [...Array(Math.max(DateStarts.length, DateEnds.length)).keys()].reduce((acc, idx) => {
        const start = DateStarts[idx];
        const end = DateEnds[idx];
        if (start !== '' || end !== '') {
            acc.push({
                'period-start': start,
                'period-end': end,
            });
        }
        return acc;
    }, []);

    for (const key of ['period-start', 'period-end']) {
        data.delete(key);
    }
    for (const pair of DatePairs) {
        for (const [key, date] of Object.entries(pair)) {
            data.append(key, date);
        }
    }

    Globals.SignaturePads.forEach( ({ key, pad }) => {
        const imageData = getSignatureImage(pad);
        if (imageData) {
            data.append(key, imageData);
        }
    });

    return data;
};

const submitFormData = (data, url) => {
    // Generate UUID per request for debugging purposes.
    const headers = new Headers();
    const uuid = uuidv4();
    headers.set('x-request-id', uuid);

    return fetch(url, {
        method: 'POST',
        headers,
        cache: 'no-cache',
        credentials: 'omit',
        redirect: 'follow',
        body: data,
    });
};

const createSignaturePads = () => {
    Globals.SignaturePads = [
        ['substitute-signature', ElementQueries.SubstituteSignatureParent],
        ['regular-signature', ElementQueries.RegularSignatureParent ],
    ].map( ([ key, query]) => {
        const el = document.querySelector(query);
        return {
            key,
            ...createSignatureInput(el),
        };
    });

    const onResizeCanvases = () => Globals.SignaturePads.forEach(({ onResize }) => onResize());
    onResizeCanvases();
};

const createUIForPopulatingWithSavedData = (form) => {
    const fieldSets = form.querySelectorAll('fieldset[data-enhanced-form-part]');
    fieldSets.forEach(fieldSet => {
        const formPart = fieldSet.dataset.enhancedFormPart;
        const legend = fieldSet.querySelector('legend');
        createPersistedDataQuickFillUI(form, formPart, fieldSet, legend);
    });
};

const setupDynamicFormChanges = (form) => {
    const dynamicLabel = form.querySelector('label[for="substitute-substitutingID"]');
    const elementsToMonitor = form.querySelectorAll('input[type=radio][name="substitute-title"]');

    const substituteNameInput = form.querySelector('#substitute-name');
    const autocompleter = new InputAutocompleter(substituteNameInput);

    elementsToMonitor.forEach(element => {
        element.addEventListener('change', event => {
            const newValue = event.target.value;
            if (newValue === 'Madame' || newValue === 'Monsieur') {
                dynamicLabel.textContent = 'Numéro de licence de remplacement:';

                autocompleter.remove();

            } else if (newValue === 'Docteur') {
                dynamicLabel.textContent = `Numéro d'inscription au tableau de l'ordre:`;

                // Setup auto-complete of names if the substitute is a doctor.
                autocompleter.setup();
            }
        });
    });
};

const setupLiveFormFeedback = (form) => {
    const NameMessage = 'Le "Nom complet" doit être renseigné.';
    const RPPSMessage = 'Le RRPS doit faire 11 chiffres.';
    const AddressMessage = `L'addresse doit être renseignée.`;
    const SIRETMessage = 'Le SIRET doit faire 14 chiffres.';
    const PeriodsMessageMap = (error) => {
        if (error === FormValidationIssues.MissingRequired) {
            return 'Il faut préciser au moins une pèriode de remplacement.';
        }
        if (error === FormValidationIssues.TooMany) {
            return 'Trop de pèriodes de remplacement, désolé.'
        }
        return "Au moins une pèriode de remplacement n'est pas valide. Il manque peut–être une date de début ou de fin ?";
    }

    const FormFeedbacks = [
        {
            name: 'regular-name',
            querySelector: '#regular-name',
            check: Validators.Required,
            overridingMesssage: NameMessage,
            errorLink: 'le nom du médecin remplacé',
        },
        {
            name: 'regular-rpps',
            querySelector: '#regular-rpps',
            check: Validators.Length(11),
            overridingMesssage: RPPSMessage,
            errorLink: 'le RPPS du médecin remplacé',
        },
        {
            name: 'regular-address',
            querySelector: '#regular-address',
            check: Validators.Required,
            overridingMesssage: AddressMessage,
            errorLink: `l'addresse du médecin remplacé`,
        },
        {
            name: 'substitute-title',
            querySelector: 'input[name="substitute-title"]',
            errorLink: 'le titre du remplaçant',
        },
        {
            name: 'substitute-name',
            querySelector: '#substitute-name',
            check: Validators.Required,
            overridingMesssage: NameMessage,
            errorLink: 'le nom du remplaçant',
        },
        {
            name: 'substitute-rpps',
            querySelector: '#substitute-rpps',
            check: Validators.Length(11),
            overridingMesssage: RPPSMessage,
            errorLink: 'le RPPS du remplaçant',
        },
        {
            name: 'substitute-siret',
            querySelector: '#substitute-siret',
            check: Validators.Length(14),
            overridingMesssage: SIRETMessage,
            errorLink: 'le SIRET du remplaçant',
        },
        {
            name: 'substitute-substitutingID',
            querySelector: '#substitute-substitutingID',
            check: Validators.Required,
            errorLink: `le numéro d'inscription au tableau / la licence de remplaçement`,
        },
        {
            name: 'substitute-address',
            querySelector: '#substitute-address',
            check: Validators.Required,
            overridingMesssage: AddressMessage,
            errorLink: `l'addresse du remplaçant`,
        },
        {
            name: 'period-start',
            querySelector: 'input[name="period-start"]',
            overridingMesssage: PeriodsMessageMap,
            errorLink: 'les dates du remplacement',
        },
        {
            name: 'period-end',
            querySelector: 'input[name="period-end"]',
            overridingMesssage: PeriodsMessageMap,
            errorLink: 'les dates du remplacement',
        },
        {
            name: 'financials-retrocession',
            querySelector: '#financials-retrocession',
            check: Validators.Number(0, 100),
            overridingMesssage: "Entre 0 et 100 s'il vous plaît !",
            errorLink: 'la rétrocession',
        },
        {
            name: 'financials-nightShiftRetrocession',
            querySelector: '#financials-nightShiftRetrocession',
            errorLink: 'la rétrocession des gardes',
        },
    ];

    return ErrorHandler(form, FormFeedbacks);
};

const setupAutocomplete = (form) => {
    const regularNameInput = form.querySelector('#regular-name');
    const autocompleter = new InputAutocompleter(regularNameInput);
    autocompleter.setup();
};

const setupUIWithin = (form) => {
    // TODO: refactor this
    createSignaturePads();
    //

    createUIForPopulatingWithSavedData(form);
    const errorHandler = setupLiveFormFeedback(form);
    createDefaultPeriodInputUI(form);

    setupAutocomplete(form);
    setupDynamicFormChanges(form);

    const extraUI = {};
    // TODO: use a wrapper to log errors on backend that are not validation errors
    extraUI.errorHandler = errorHandler;

    return extraUI;
};

const onDOMContentLoaded = () => {
    polyfill();

    const form = document.querySelector('form');
    const extraUI = setupUIWithin(form);
    setupFormIntercept(form, extraUI);
};

const onDOMReady = (callback) => {
    const ready = document.readyState === "interactive" || document.readyState === "complete";
    if (ready) {
        callback();
        return;
    }
    document.addEventListener('DOMContentLoaded', callback);
};

window.addEventListener('unhandledrejection', (event) => {
    console.info("App INFO 'unhandledrejection'", event.reason);
    // TODO: send analytics
});

window.addEventListener('error', (event) => {
    // TODO: first thing, try to send analytics
    const { error } = event;
    console.info("App INFO 'error'", event);

    // TODO: show the user some feedback and reassure them we're looking into it.
    // e.g. "Désolé pour ce contre-temps ! À priori, nous allons être notifié et essayer de régler le problème mais n'hésitez pas à nous envoyer un email avec l'erreur que vous venez de recontrer."
});

onDOMReady(onDOMContentLoaded);
