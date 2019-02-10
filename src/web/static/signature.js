import { makeElement } from './utils';
import SignaturePad from 'signature_pad';

// from https://github.com/szimek/signature_pad/blob/gh-pages/js/app.js
// Adjust canvas coordinate space taking into account pixel ratio,
// to make it look crisp on mobile devices.
// This also causes canvas to be cleared.
const onResizeCanvas = (canvas, signaturePad) => {
    // When zoomed out to less than 100%, for some very strange reason,
    // some browsers report devicePixelRatio as less than 1
    // and only part of the canvas is cleared then.
    var ratio =  Math.max(window.devicePixelRatio || 1, 1);

    // This part causes the canvas to be cleared
    canvas.width = canvas.offsetWidth * ratio;
    canvas.height = canvas.offsetHeight * ratio;
    canvas.getContext('2d').scale(ratio, ratio);

    // This library does not listen for canvas changes, so after the canvas is automatically
    // cleared by the browser, SignaturePad#isEmpty might still return false, even though the
    // canvas looks empty, because the internal data of this library wasn't cleared. To make sure
    // that the state of this library is consistent with visual state of the canvas, you
    // have to clear it manually.
    signaturePad.clear();
};

export const createSignatureInput = (parentElement) => {
    const labelWrapper = makeElement('div', el => {
        el.classList.add('d-flex', 'flex-row', 'signature-pad-label-container');
    });
    const label = makeElement('label', el => {
        el.textContent = 'Signature';
        const boldSpan = makeElement('span', el => {
            el.textContent = ' (Optionnel):';
            el.classList.add('text-bold');
        });
        el.appendChild(boldSpan);
    });
    const wrapper = makeElement('div', el => {
        el.classList.add('d-flex', 'signature-pad-container');
    });
    const canvas = makeElement('canvas', el => {
        el.classList.add('user-input', 'signature-pad');
    });

    const pad = new SignaturePad(canvas);

    const clearButton = makeElement('button', el => {
        el.classList.add('small');
        el.setAttribute('type', 'button');
        el.textContent = 'Effacer';
        el.addEventListener('click', (e) => {
            e.preventDefault();
            pad.clear();
        });
    });

    parentElement.appendChild(labelWrapper);
    labelWrapper.appendChild(label);
    labelWrapper.appendChild(clearButton);
    parentElement.appendChild(wrapper);
    wrapper.appendChild(canvas);

    return {
        pad,
        onResize: () => onResizeCanvas(canvas, pad),
    };
}

export const getSignatureImage = (signaturePad) => {
    if (signaturePad.isEmpty()) {
        return null;
    }
    return signaturePad.toDataURL("image/svg+xml");
}
