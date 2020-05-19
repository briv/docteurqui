// A backwards-compatible (striving till IE11) JS file to log unhandled errors.
(function () {

    function remoteLogError(error, eventType) {
        if (!error) {
            return;
        }

        var userAgent;
        if (window.navigator && window.navigator.userAgent) {
            userAgent = window.navigator.userAgent;
        }

        if (!userAgent) {
            return;
        }

        var message;
        if (error.message) {
            message = error.message;
        } else if (error.reason) {
            // Use .reason for events of type 'unhandledrejection'.
            message = error.reason;
        } else if (error.toString) {
            message = error.toString();
        }

        if (!message) {
            return;
        }

        var stack;
        if (error.stack) {
            stack = error.stack;
        }

        var body = 'message=' + encodeURIComponent(message);
        body += '&eventType=' + encodeURIComponent(eventType);
        body += '&useragent=' + encodeURIComponent(userAgent);
        if (stack) {
            body += '&stack=' + encodeURIComponent(stack);
        } else {
            // We don't have a stack trace, but try to create something.
            if (error.filename !== undefined && error.lineno !== undefined && error.colno !== undefined) {
                var barebonesStack = error.filename + ':' + error.lineno.toString() + ':' + error.colno.toString();
                body += '&stack=' + encodeURIComponent(barebonesStack);
            }
        }

        var url = window.location + 'b/log-error';
        var req = new XMLHttpRequest();
        req.open('POST', url);
        req.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
        req.send(body);
    }

    function unhandledErrorEventHandler(event, eventType) {
        // Do not call preventDefault(). In case we mess up remote logging, something will at
        // least show up locally on the user's console and possibly allow them to alert us.

        // Send the error to our backend for analysis.
        if (event.error) {
            remoteLogError(event.error, eventType);
        } else {
            remoteLogError(event, eventType);
        }
    };

    if (!window || !window.addEventListener) {
        return;
    }

    window.addEventListener('error', function (event) {
        // Not likely, but use try/catch just in case we'd end up in a loop for some browser implementations.
        try {
            unhandledErrorEventHandler(event, event.type);
        } catch (err) {}
    });

    window.addEventListener('unhandledrejection', function (event) {
        try {
            unhandledErrorEventHandler(event, event.type);
        } catch (err) {}
    });
})();
