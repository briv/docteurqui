import { v4 as uuidv4 } from 'uuid';

import { makeElement } from './utils';
import { createSignatureInput, getSignatureImage } from './signature';
import { saveFilledFormData, createPreviousDataInputUI } from './form-fill';
import { setupFieldFeedback, Validators } from './live-form-feedback';
import { createSinglePeriodInput, parseFormattedFRDate } from './periods-input';

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
    const fieldSetElement = form.querySelector('.date-range-input');

    const addDateInputButton = makeElement('button', el => {
        el.classList.add('small');
        el.setAttribute('type', 'button');
        el.textContent = 'Ajouter une autre pèriode';
    });
    fieldSetElement.appendChild(addDateInputButton);

    createSinglePeriodInput(fieldSetElement, addDateInputButton);

    addDateInputButton.addEventListener('click', event => {
        event.preventDefault();
        createSinglePeriodInput(fieldSetElement, addDateInputButton);
    });
};

const setupFormIntercept = (form, UI) => {
    form.addEventListener('submit', (e) => {
        if (e.preventDefault) {
            e.preventDefault();
        }

        const formData = new FormData(form);
        const url = form.action;
        const submission = submitFormData(formData, url)
            .then(response => {
                if (response.ok) {
                    return response;
                } else {
                    throw new Error('TODO, get detailed error code to show useful info to user');
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
            });

        submission.catch(e => UI.errorHandler(e));

        submission.then(() => {
            // If submission succeeds, serialize form data and save it locally.
            saveFilledFormData(formData);
        }).catch(() => {
            // Otherwise, do nothing. This prevents triggering the 'unhandledrejection' event.
        });

        return false;
    })
};

const submitFormData = (data, url) => {
    // special processing for dates within period-start and period-end
    const processDates = (key) => {
        return data.getAll(key)
            .map(el => {
                try {
                    const date = parseFormattedFRDate(el);
                    return `${date.getFullYear()}-${(date.getMonth() +1).toString().padStart(2, '0')}-${date.getDate().toString().padStart(2, '0')}`;
                } catch (e) {
                    return null;
                }
            })
            .filter(el => el != null);
    };
    for (let key of ['period-start', 'period-end']) {
        const dates = processDates(key);
        data.delete(key);
        for (let e of dates) {
            data.append(key, e);
        }
    }

    Globals.SignaturePads.forEach( ({ key, pad }) => {
        const imageData = getSignatureImage(pad);
        if (imageData) {
            data.append(key, imageData);
        }
    });

    // Generate UUID per request for debugging purposes.
    const headers = new Headers();
    try {
        const uuid = uuidv4();
        headers.set('x-request-id', uuid);
    } catch (e) {
        // TODO: use our own 'ClientError' type here ??
        return Promise.reject(e);
    }

    return fetch(url, {
        method: 'POST',
        headers,
        cache: 'no-cache',
        credentials: 'omit',
        redirect: 'follow',
        body: data,
    })
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
        createPreviousDataInputUI(formPart, fieldSet, legend);
    });
};

const setupDynamicFormChanges = (form) => {
    const dynamicLabel = form.querySelector('label[for="substitute-substitutingID"]');
    const elementsToMonitor = form.querySelectorAll('input[type=radio][name="substitute-title"]');

    elementsToMonitor.forEach(element => {
        element.addEventListener('change', event => {
            const newValue = event.target.value;
            if (newValue === 'Madame' || newValue === 'Monsieur') {
                dynamicLabel.textContent = 'Numéro de licence de remplacement:';
            } else if (newValue === 'Docteur') {
                dynamicLabel.textContent = `Numéro d'inscription au tableau de l'ordre:`;
            }
        });
    });
};

const formErrorHandler = (form) => (error) => {
    const potentialErrorContainer = form.querySelector('.call-to-action.error');
    // const submitButton = form.querySelector('input[type="submit"]');
    if (!potentialErrorContainer) {
        const div = makeElement('div', el => {
            el.classList.add('call-to-action', 'error');
        });
        form.appendChild(div);
    }

    const errorContainer = form.lastChild;
    while (errorContainer.firstChild) {
        errorContainer.removeChild(errorContainer.lastChild);
    }
    errorContainer.appendChild(makeElement('p', el => {
        // TODO: this text should tell the user what went wrong, along with links to jump
        // to the correct spot in the page.
        // TODO: For errors with the date input system, we should have a little error explanation underneath as well !
        el.textContent = `An error: "${error}"`;
        el.classList.add('error-animation');
    }));

    // TODO: smooth scroll polyfill !
    const scrollArg = ('scrollBehavior' in document.documentElement.style) ?
        { behavior: 'smooth', block: 'start', inline: 'nearest' } : true;
    errorContainer.scrollIntoView(scrollArg);
};

const setupLiveFormFeedback = (form) => {
    const FormFeedbackParentQuerySelector = '.single-form-input-group';

    const NameMessage = 'Le "Nom complet" doit être renseigné.';
    const RPPSMessage = 'Le RRPS doit faire 11 chiffres.';

    const formFeedbacks = [
        {
            querySelector: 'input#regular-name',
            check: Validators.Required,
            message: NameMessage,
        },
        {
            querySelector: 'input#regular-rpps',
            check: Validators.Length(11),
            message: RPPSMessage,
        },
        {
            querySelector: 'input#substitute-name',
            check: Validators.Required,
            message: NameMessage,
        },
        {
            querySelector: 'input#substitute-rpps',
            check: Validators.Length(11),
            message: RPPSMessage,
        },
        {
            querySelector: 'input#substitute-substitutingID',
            check: Validators.Required,
            message: 'Ce champ doit être renseigné.',
        },
        {
            querySelector: 'input[name="financials-retrocession"]',
            check: Validators.Number(0, 100),
            message: "Entre 0 et 100 s'il vous plaît !",
        },
    ];
    formFeedbacks.forEach(el => { setupFieldFeedback(form, FormFeedbackParentQuerySelector, el) });
};

const setupUIWithin = (form) => {
    // TODO: refactor this
    createSignaturePads();
    //

    createUIForPopulatingWithSavedData(form);
    setupLiveFormFeedback(form);
    createDefaultPeriodInputUI(form);

    setupDynamicFormChanges(form);

    const extraUI = {};
    extraUI.errorHandler = formErrorHandler(form);

    return extraUI;
};

const onDOMContentLoaded = () => {
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
    console.info("App INFO 'error'", event.reason);
    // TODO: send analytics
});

onDOMReady(onDOMContentLoaded);
