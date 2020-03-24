import { makeElement } from './utils';

const StorageFormDataVersion = '1';
const StorageFormDataKey = `savedFormData-${StorageFormDataVersion}`;

const FormParts = {
    Regular: 'regular',
    Substitute: 'substitute',
};

const FormDataKeySuffixes = {
    Primary: 'rpps',
    Name: 'name',
    Title: 'title',
};
const FormDataKeySuffixWhiteList = {
    [FormParts.Regular]: (new Set([
        FormDataKeySuffixes.Primary,
        FormDataKeySuffixes.Name,
        'adeli',
        'address',
    ])),
    [FormParts.Substitute]: (new Set([
        FormDataKeySuffixes.Primary,
        FormDataKeySuffixes.Name,
        FormDataKeySuffixes.Title,
        'substitutingID',
        'address',
    ])),
};

const fillFormField = (nameAttribute, value) => {
    const elements = [...document.querySelectorAll(`form *[name=${nameAttribute}]`).values()];
    if (elements.length > 1) {
        // Check that all elements are <input[type="radio"]>.
        const check = elements.every(el =>
            el.tagName === 'INPUT'
            && el.getAttribute('type') === 'radio'
        );
        if (check === false) {
            // TODO: this should be prettier so the user can see it (in case).
            throw new Error(`UH OH ${elements.length} found for query selector [name="${nameAttribute}"]`);
        }

        elements.forEach(el => {
            if (el.value === value) {
                el.checked = true;
                el.dispatchEvent(new Event('change'));
            } else {
                el.checked = false;
            }
        });

        return;
    }


    const element = elements[0];
    const tagName = element.tagName;

    if (tagName === 'INPUT') {
        element.value = value;
        element.dispatchEvent(new Event('input'));
        return;
    }
};

const storeAllData = (allData) => {
    const serializedData = JSON.stringify(allData);
    if (serializedData === '{}' && window.localStorage.getItem(StorageFormDataKey) === null) {
        return;
    }
    window.localStorage.setItem(StorageFormDataKey, serializedData);
};

const retrieveAllSavedData = () => {
    const serializedData = window.localStorage.getItem(StorageFormDataKey);
    if (serializedData === null) {
        return {};
    }

    const data = JSON.parse(serializedData);
    return data;
};

const previouslyEnteredData = (formPart) => {
    const allData = retrieveAllSavedData();
    return allData[formPart] || null;
};

const onDeletePreviousData = (event, rootElement) => {
    event.preventDefault();
    const currentTarget = event.currentTarget;
    const rowForData = currentTarget.closest('.profile-wrapper');

    const selectedValue = rowForData.dataset.formValue;
    const formPart = rowForData.dataset.enhancedFormPart;

    rowForData.remove();
    // Remove the entire UI if all previous data is deleted
    if (rootElement.querySelectorAll('.profile-wrapper').length === 0) {
        rootElement.remove();
    }

    // Remove the locally saved data that the user wants to delete.
    const previousData = previouslyEnteredData(formPart);
    if (!previousData) {
        return;
    }
    const newParts = previousData.filter(datum => datum[FormDataKeySuffixes.Primary] !== selectedValue);
    const newData = { ...retrieveAllSavedData(), [formPart]: newParts };
    storeAllData(newData);
};

const onSelectPreviousData = (event) => {
    const currentTarget = event.currentTarget;
    const rowForData = currentTarget.closest('.profile-wrapper');
    const formPart = rowForData.dataset.enhancedFormPart;
    const previousData = previouslyEnteredData(formPart);
    if (!previousData) {
        return;
    }
    const chosenValue = rowForData.dataset.formValue;
    const partToFill = previousData.find(datum => datum[FormDataKeySuffixes.Primary] === chosenValue);
    if (!partToFill) {
        return;
    }
    Object.entries(partToFill).forEach(([suffix, value]) => {
        fillFormField(`${formPart}-${suffix}`, value);
    });
};

export const createPreviousDataInputUI = (formPart, parentNode, siblingChildNode) => {
    const previousData = previouslyEnteredData(formPart);
    if (!previousData || previousData.length === 0) {
        return;
    }

    const rootDiv = makeElement('div', el => {
        el.classList.add('call-to-action');
    });
    const p = makeElement('p', el => {
        el.textContent = 'Pour réutiliser des informations précédemment renseignées et éviter de les re-taper, sélectionnez parmi les personnes déjà enregistrées.';
    });
    const profilesManagement = makeElement('div', el => {
        el.classList.add('d-flex', 'flex-column', 'existing-profiles');

        previousData.forEach(profile => {
            const wrapper = makeElement('div', el => {
                el.classList.add('d-flex', 'flex-row', 'profile-wrapper');

                const option = makeElement('div', el => {
                    el.classList.add('d-flex', 'flex-row', 'profile-option');

                    el.setAttribute('tabindex', '0');

                    const s = makeElement('span', el => {
                        const name = profile[FormDataKeySuffixes.Name] || '';
                        el.textContent = formPart == FormParts.Regular ? `Dr. ${name}` : `${name}`;
                    });
                    el.appendChild(s);
                });

                const deleteButton = makeElement('button', el => {
                    el.classList.add('small');
                    el.setAttribute('type', 'button');
                    el.textContent = '❌';
                });

                el.appendChild(option);
                el.appendChild(deleteButton);

                el.dataset.enhancedFormPart = formPart;
                const id = profile[FormDataKeySuffixes.Primary];
                el.dataset.formValue = id;

                option.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter') {
                        onSelectPreviousData(e);
                    }
                });

                el.addEventListener('click', (e) => {
                    onSelectPreviousData(e);
                });
                deleteButton.addEventListener('click', (e) => {
                    onDeletePreviousData(e, rootDiv);
                });
            });

            el.appendChild(wrapper);
        });
    });

    rootDiv.appendChild(p);
    rootDiv.appendChild(profilesManagement);
    parentNode.insertBefore(rootDiv, siblingChildNode.nextSibling);
};

export const saveFilledFormData = (formData) => {
    if (!window.localStorage) {
        return;
    }

    const finalFormParts = Object.fromEntries(
        Object.values(FormParts)
            .map(formPartKeyPrefix => {
                // Do not save this subset of the form data unless we have a primary key to identify it.
                const enteredPrimaryKey = formData.get(`${formPartKeyPrefix}-${FormDataKeySuffixes.Primary}`);
                if (!enteredPrimaryKey) {
                    return [];
                }

                const suffixWhiteList = FormDataKeySuffixWhiteList[formPartKeyPrefix];
                if (!suffixWhiteList || suffixWhiteList.size === 0) {
                    return [];
                }

                const data = {};
                for (const [, suffix] of suffixWhiteList.entries()) {
                    const formName = `${formPartKeyPrefix}-${suffix}`;
                    const value = formData.get(formName);
                    if (value) {
                        data[suffix] = value;
                    }
                }
                const previousParts = previouslyEnteredData(formPartKeyPrefix) || [];
                const newParts = [data, ...previousParts.filter(el => el[FormDataKeySuffixes.Primary] !== enteredPrimaryKey) ];

                return [ formPartKeyPrefix, newParts ];
            })
            .filter(value => value.length !== 0)
        );

    const newData = { ...retrieveAllSavedData(), ...finalFormParts };
    storeAllData(newData);
};
