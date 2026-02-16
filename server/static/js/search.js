let ws;
let input = document.getElementById("search");
let autocompleteEl = document.getElementById("autocomplete");
let results = document.getElementById("results");
let csrf = document.getElementById("csrf_token");
let statusEl = document.getElementById("ws-status");
let emptyImg = "data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==";
let urlState = {};
let lastResults = null;
let templates = {};
for(let el of document.querySelectorAll("template")) {
    let id = el.getAttribute("id")
    templates[id] = el;
}

const hotkeys = JSON.parse(document.getElementById("hotkey-data").text);

const hotkeyActions = {
    "open_result": openSelectedResult,
    "open_result_in_new_tab": e => openSelectedResult(e, true),
    "select_next_result": selectNextResult,
    "select_previous_result": selectPreviousResult,
    "open_query_in_search_engine": openQueryInSearchEngine,
    "focus_search_input": focusSearchInput,
    "view_result_popup": viewResultPopup,
    "autocomplete": autocomplete,
    "show_hotkeys": showHotkeys,
}

const hotkeyDescriptions = {
    "open_result": "Open result",
    "open_result_in_new_tab": "Open result in new tab",
    "select_next_result": "Select next result",
    "select_previous_result": "Select previous result",
    "open_query_in_search_engine": "Open query in search engine",
    "focus_search_input": "Focus search input",
    "view_result_popup": "View result popup",
    "autocomplete": "Autocomplete query",
    "show_hotkeys": "Show Hotkeys",
}

const tips = [
    'Use <code>*</code> for partial match.<br />Prefixing word with <code>-</code> excludes matching documents.',
    'Click on the three dots near the result URL to specify priority queries for that result.',
    'Press <code>enter</code> to open the first result.',
    'Use <code>alt+k</code> and <code>alt+j</code> to navigate between results.', // TODO replace keybindings to configured hotkeys
    'Press <code>alt+o</code> to open current search query in your configured search engine.',
    'Use <code>url:</code> prefix to search only in the URL field. E.g.: <code>url:*github* hister</code>.', // TODO replace keybindings to configured hotkeys
    'Set hister to your default search engine in your browser to access it with ease.',
    'Start search query with <code>!!</code> to open the query in your configured search engine',
];

function createTemplate(name, fns) {
    let el = document.importNode(templates[name].content, true)
    for(let k in fns) {
        fns[k](el.querySelector(k));
    }
    return el;
}

function connect() {
    ws = new WebSocket(document.querySelector("#ws-url").value);

    ws.onopen = function() {
        updateConnectionStatus(true);
        const urlParams = new URLSearchParams(window.location.search);
		const query = urlParams.get('q');
        if(query) {
            sendQuery(query);
            input.value = query;
        }
    };

    ws.onmessage = renderResults;

    ws.onclose = function() {
        console.log("WebSocket connection closed, retrying...");
        updateConnectionStatus(false);
        setTimeout(connect, 1000); // Reconnect after 1 second
    };

    ws.onerror = function(error) {
        console.error("WebSocket error:", error);
        updateConnectionStatus(false);
    };
}

function updateConnectionStatus(connected) {
    if(connected) {
        statusEl.classList.add("connected");
        statusEl.title = "Websocket connected";
    } else {
        statusEl.classList.remove("connected");
        statusEl.title = "Websocket disconnected";
    }
}

function sendQuery(q) {
    let message = {"text": q, "highlight": "HTML"};
    ws.send(JSON.stringify(message));
}

function updateURL() {
    if(input.value) {
        history.replaceState(urlState, "", `${window.location.pathname}?q=${encodeURIComponent(input.value)}`);
        return;
    }
    history.replaceState(urlState, "", `${window.location.pathname}`);
}

function renderResults(event) {
    const res = JSON.parse(event.data);
    lastResults = res;
    const d = res.documents;
    updateAutocomplete(res.query_suggestion);
    if(!d && !res.history) {
        if(!input.value) {
            results.replaceChildren(createTips());
            return
        }
        let u = getSearchUrl(input.value)
        let n = createTemplate("result", {
            "a": (e) => { e.setAttribute("href", u); e.innerHTML = "No results found - open query in web search engine"; e.classList.add("error"); },
            ".result-url": (e) => { e.textContent = u; },
        });
        results.replaceChildren(n);
        return;
    }
    let resultElements = [];
    highlightIdx = 0;
    resultElements.push(createResultsHeader(res));
    if(res.history && res.history.length) {
        for(let r of res.history) {
            resultElements.push(createPriorityResult(r))
        }
    }
    if(d) {
        for(let r of d) {
            resultElements.push(createResult(r));
        }
    }
    results.replaceChildren(...resultElements);
    if(resultElements.length > 0) {
        let r = document.querySelector(".result");
        if(r) {
            r.classList.add("highlight");
        }
    }
    scrollTo(results.children[0]);
};

input.addEventListener("input", () => {
    handleInput();
});

function handleInput() {
    updateURL();
    sendQuery(input.value);
}

function createTips() {
    return createTemplate("tips", {
        ".content": e => e.innerHTML = tips[Math.floor(Math.random() * tips.length)],
    });
}

function getSearchUrl(query) {
    return document.querySelector("#search-url").value.replace("{query}", escape(query));
}

function updateAutocomplete(suggestion) {
    if(!input.value) {
        autocompleteEl.value = "Search...";
        return;
    }
    if(!suggestion) {
        autocompleteEl.value = "";
        return;
    }
    autocompleteEl.value = suggestion.replaceAt(0, input.value);
}

function openUrl(u, newWindow) {
    if(newWindow) {
        window.open(u, '_blank');
        window.focus();
        return;
    }
    window.location.href = u;
}

function formatTimestamp(unixTimestamp) {
    return new Date(unixTimestamp * 1000).toISOString().replace("T", " ").split(".")[0];
}

function formatRelativeTime(unixTimestamp) {
    if(!unixTimestamp) {
        return '';
    }

    const now = Date.now();
    const timestamp = unixTimestamp * 1000;
    const secondsAgo = Math.floor((now - timestamp) / 1000);

    if(secondsAgo < 0) {
        return 'just now';
    }

    if(secondsAgo < 60) {
        return 'just now';
    }

    const minutesAgo = Math.floor(secondsAgo / 60);
    if(minutesAgo < 60) {
        return minutesAgo === 1 ? '1 minute ago' : `${minutesAgo} minutes ago`;
    }

    const hoursAgo = Math.floor(minutesAgo / 60);
    if(hoursAgo < 24) {
        return hoursAgo === 1 ? '1 hour ago' : `${hoursAgo} hours ago`;
    }

    const daysAgo = Math.floor(hoursAgo / 24);
    if(daysAgo < 7) {
        return daysAgo === 1 ? 'yesterday' : `${daysAgo} days ago`;
    }

    const weeksAgo = Math.floor(daysAgo / 7);
    if(weeksAgo < 4) {
        return weeksAgo === 1 ? '1 week ago' : `${weeksAgo} weeks ago`;
    }

    const monthsAgo = Math.floor(daysAgo / 30);
    if(monthsAgo < 12) {
        return monthsAgo === 1 ? '1 month ago' : `${monthsAgo} months ago`;
    }

    const yearsAgo = Math.floor(daysAgo / 365);
    return yearsAgo === 1 ? '1 year ago' : `${yearsAgo} years ago`;
}

function init() {
    results.replaceChildren(createTips());
    connect();

    const hotkeyButton = document.getElementById('hotkey-button');
    hotkeyButton.addEventListener('click', showHotkeys);

    const hideButton = localStorage.getItem('hideHotkeyButton');
    if(hideButton === 'true') {
        hotkeyButton.classList.add('hidden');
    }
}

function openResult(e, newWindow) {
    if(e.preventDefault) {
        e.preventDefault();
    }
    if (e.ctrlKey) {
        newWindow = true;
    }
    let link = e.target.closest('.result-title a')
    let url = link.getAttribute("href");
    let title = link.innerText;
    if(!link.classList.contains("error")) {
        saveHistoryItem(url, title, input.value, false, r => openUrl(url, newWindow));
    } else {
        openUrl(url, newWindow);
    }
    return false;
}

function createResultsHeader(res) {
    const d = res.documents;
    const header = createTemplate("results-header", {
        ".duration": (e) => e.innerText = res.search_duration || "",
        ".results-num": (e) => e.innerText = res.total || d.length,
        ".export-json": (e) => e.addEventListener("click", () => exportJSON()),
        ".export-csv": (e) => e.addEventListener("click", () => exportCSV()),
        ".export-rss": (e) => e.addEventListener("click", () => exportRSS()),
    });
    if(res.query && res.query.text != input.value) {
        header.querySelector(".expanded-query").innerHTML = `Expanded query: <code>"${escapeHTML(res.query.text)}"</code>`;
    }
    return header;
}

function escapeHTML(s) {
    let pre = document.createElement('pre');
    let text = document.createTextNode(s);
    pre.appendChild(text);
    return pre.innerHTML;
}

function createPriorityResult(r) {
    let rn = createTemplate("result", {
        "a": (e) => {
            e.setAttribute("href", r.url);
            e.innerHTML = r.title || "*title*";
            clickHandler(e, openResult, ev => openResult(ev, true));
            e.classList.add("success");
        },
        ".readable": e => createReadable(e, r.url),
        ".result-url": (e) => { e.textContent = r.url; },
        ".action-button": e => e.addEventListener("click", (ev) => togglePriorityActions(ev, e.closest(".result"))),
    });
    return rn;
}

function createResult(r) {
    let rn = createTemplate("result", {
        ".result-title a": e => {
            e.setAttribute("href", r.url);
            e.innerHTML = r.title || "*title*";
            clickHandler(e, openResult, ev => openResult(ev, true));
        },
        ".readable": e => createReadable(e, r.url),
        "img": e => e.setAttribute("src", r.favicon || emptyImg),
        ".result-url": e => e.textContent = r.url,
        ".added": e => {
            e.textContent = formatRelativeTime(r.added);
            e.title = formatTimestamp(r.added);
        },
        ".action-button": e => e.addEventListener("click", (ev) => toggleActions(ev, e.closest(".result"))),
        "p": e => e.innerHTML = r.text || "",
    });
    return rn;
}

function clickHandler(e, leftClickCallback, middleClickCallback) {
    e.addEventListener("click", ev => {
        leftClickCallback(ev);
    });
	e.addEventListener("auxclick", ev => {
		if (ev.button == 1) {
			middleClickCallback(ev);
		}
	});

}

function createReadable(e, u) {
    e.setAttribute("data-href", "/readable?url="+encodeURIComponent(u));
    e.addEventListener("click", openReadable);
}

function openReadable(e) {
    let result = e.target.closest(".result");
    let link = result.querySelector(".result-title a");
    let url = link.getAttribute("href");
    let title = link.innerText;
    let h = `<h1><a href="${url}">${title}</a></h1>`;
    request(
        e.target.getAttribute("data-href"),
        {},
        resp => resp.text().then(t => openPopup(h, t))
    );
    return false;
}

function openPopup(header, content) {
    closePopup();
    let close = e => e.addEventListener("click", ev => {
        if(ev.target.classList.contains("popup-close") || ev.target.classList.contains("popup-wrapper")) {
            closePopup();
        }
    });
    let p = createTemplate("popup", {
        ".popup-wrapper": close,
        ".popup-close": close,
        ".popup": e => e.addEventListener("click", ev => false),
        ".popup-header": e => e.innerHTML = header,
        ".popup-content": e => e.innerHTML = content,
    });
    document.body.appendChild(p);
}

function closePopup() {
    let p = document.querySelector(".popup-wrapper");
    if(p) {
        p.remove();
        return true;
    }
    return false;
}

function togglePriorityActions(ev, res) {
    let a = res.querySelector(".actions")
    if(a) {
        closeActions(a);
        return;
    }
    a = createTemplate("priority-actions", {
        ".delete": (e) => e.addEventListener("click", () => updatePriorityResult(e, true)),
        ".close": (e) => e.addEventListener("click", () => closeActions(e)),
    });
    for(let e of a.children) {
        e.style.animation = "fade-in 0.5s";
    }
    res.appendChild(a);
}

function toggleActions(ev, res) {
    let a = res.querySelector(".actions")
    if(a) {
        closeActions(a);
        return;
    }
    a = createTemplate("result-actions", {
        ".save": (e) => e.addEventListener("click", () => updatePriorityResult(e, false)),
        ".close": (e) => e.addEventListener("click", () => closeActions(e)),
        ".delete": (e) => e.addEventListener("click", () => deleteResult(e)),
    });
    for(let e of a.children) {
        e.style.animation = "fade-in 0.5s";
    }
    res.appendChild(a);
}

function closeActions(e) {
    e.closest(".actions").remove();
}

function updatePriorityResult(e, remove) {
    let result = e.closest(".result");
    let link = result.querySelector(".result-title a");
    let url = link.getAttribute("href");
    let title = link.innerText;
    let query = input.value;
    let queryEl = result.querySelector(".action-query");
    if(queryEl && queryEl.value) {
        query = queryEl.value;
    }
    if(!query) {
        return;
    }
    saveHistoryItem(url, title, query, remove, r => {
        let tpl;
        if(r.status == 200) {
            tpl = createTemplate("success", {
                ".message": (e) => e.innerText = `Priority result ${remove ? "deleted" : "added"}.`,
            });
        } else {
            tpl = createTemplate("error", {
                ".message": (e) => e.innerText = `Failed to ${remove ? "delete" : "add"} priority result.`,
            });
        }

        result.querySelector(".actions").appendChild(tpl);
    });
}

function deleteResult(e) {
    let result = e.closest(".result");
    let url = result.querySelector(".result-title a").getAttribute("href");
    let data = new URLSearchParams({"url": url});
    request(
        "/delete",
        {
            method: "POST",
            body: data,
        },
        r => result.remove()
    );
}

function saveHistoryItem(url, title, query, remove, callback) {
    return request(
        "/history",
        {
            method: "POST",
            body: JSON.stringify({"url": url, "title": title, "query": query, "delete": remove}),
            headers: {"Content-type": "application/json; charset=UTF-8" },
        },
        callback
    );
}

let highlightIdx = 0;
window.addEventListener("keydown", function(e) {
    if(!e.key) {
        return;
    }
    let modifier;
    if(e.altKey) {
        modifier = "alt";
    } else if(e.ctrlKey) {
        modifier = "ctrl";
    } else if(e.metaKey) {
        modifier = "meta";
    }
    let key = e.key.toLowerCase();
    if(modifier) {
        key = modifier + "+" + key;
    }
    if(hotkeys[key]) {
        hotkeyActions[hotkeys[key]](e);
        return;
    }
    if(e.key == 'Escape') {
        if(closePopup()) {
            e.preventDefault();
            return;
        }
    }
});

function autocomplete(e) {
    if(document.activeElement == input && input.value != autocompleteEl.value) {
        input.value = autocompleteEl.value;
        handleInput();
        e.preventDefault();
        return;
    }
}

function viewResultPopup(e) {
    e.preventDefault();
    if(!closePopup()) {
        openReadable({'target': document.querySelectorAll(".result .readable")[highlightIdx]});
    }
}

function openSelectedResult(e, newWindow) {
    if(input.value.startsWith("!!")) {
        openUrl(getSearchUrl(input.value.substring(2)), newWindow);
        return;
    }
    e.preventDefault();
    let res = document.querySelectorAll(".result .result-title a")[highlightIdx];
    openResult({'target': res}, newWindow);
}

function selectNextResult(e) {
    selectNthResult(e, 1);
}

function selectPreviousResult(e) {
    selectNthResult(e, -1);
}

function selectNthResult(e, n) {
    e.preventDefault();
    let res = document.querySelectorAll(".result");
    if(res.length > highlightIdx) {
        res[highlightIdx].classList.remove("highlight");
    }
    highlightIdx = (highlightIdx+n+res.length) % res.length;
    res[highlightIdx].classList.add("highlight");
    scrollTo(res[highlightIdx]);
}

function scrollTo(el) {
    let staticOffset = 60;
    let searchRect = document.querySelector('.search').getBoundingClientRect()
    let topPos =  searchRect.height + searchRect.y;
    let rect = el.getBoundingClientRect();
    if(rect.y <= topPos) {
        let offset = rect.y - topPos - staticOffset;
        window.scrollBy(0, offset);
        return;
    }
    if(rect.y+rect.height > window.innerHeight-staticOffset) {
        let offset = rect.y+rect.height - window.innerHeight + staticOffset;
        window.scrollBy(0, offset);
        return;
    }
}

function openQueryInSearchEngine(e) {
    e.preventDefault();
    openUrl(getSearchUrl(input.value));
}

function focusSearchInput(e) {
    if(document.activeElement != input) {
        e.preventDefault();
        input.focus();
        return;
    }
}

function showHotkeys(e) {
    if(document.activeElement == input) {
        return;
    }
    if(closePopup()) {
        return;
    }
    let c = document.createElement('div');
    for(let k in hotkeys) {
        let t = createTemplate("hotkey", {
            "kbd": e => e.textContent = k,
            "span": e => e.textContent = hotkeyDescriptions[hotkeys[k]],
        });
        c.appendChild(t);
    }

    const hideButton = localStorage.getItem('hideHotkeyButton') === 'true';
    const toggleSection = `
        <div class="hotkey-toggle-section mt-1">
            <p>The hotkey button can be toggled using the button below. You can always press <kbd>?</kbd> to view this popup.</p>
            <button class="hotkey-toggle-btn mt-1" onclick="toggleHotkeyButton()">
                ${hideButton ? 'Show Hotkey Button' : 'Hide Hotkey Button'}
            </button>
        </div>
    `;

    openPopup("<h2>Hotkeys</h2>", c.innerHTML + toggleSection);
}

function toggleHotkeyButton() {
    const hotkeyButton = document.getElementById('hotkey-button');
    const isHidden = localStorage.getItem('hideHotkeyButton') === 'true';

    if(isHidden) {
        localStorage.setItem('hideHotkeyButton', 'false');
        hotkeyButton.classList.remove('hidden');
    } else {
        localStorage.setItem('hideHotkeyButton', 'true');
        hotkeyButton.classList.add('hidden');
    }

    closePopup();
    showHotkeys();
}

function downloadFile(content, filename, mimeType) {
    let blob = new Blob([content], {type: mimeType});
    let a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    a.click();
    URL.revokeObjectURL(a.href);
}

function exportJSON() {
    if(!lastResults) return;
    downloadFile(JSON.stringify(lastResults, null, 2), "results.json", "application/json");
}

function exportCSV() {
    if(!lastResults) return;
    let rows = [["url", "title", "domain", "score"]];
    if(lastResults.documents) {
        for(let d of lastResults.documents) {
            rows.push([d.url, d.title, d.domain, d.score]);
        }
    }
    let csv = rows.map(r => r.map(v => '"' + String(v || "").replace(/"/g, '""') + '"').join(",")).join("\n");
    downloadFile(csv, "results.csv", "text/csv");
}

function exportRSS() {
    if(!lastResults) return;
    let items = "";
    if(lastResults.documents) {
        for(let d of lastResults.documents) {
            items += `<item><title>${escapeXML(d.title || "")}</title><link>${escapeXML(d.url || "")}</link></item>`;
        }
    }
    let rss = `<?xml version="1.0" encoding="UTF-8"?>\n<rss version="2.0"><channel><title>Hister search: ${escapeXML(input.value)}</title>${items}</channel></rss>`;
    downloadFile(rss, "results.rss", "application/rss+xml");
}

function escapeXML(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&apos;");
}

String.prototype.replaceAt = function(index, replacement) {
    return this.substring(0, index) + replacement + this.substring(index + replacement.length);
}

function request(url, params, callback) {
    if(!params) {
        params = {};
    }
    if(!params.headers) {
        params.headers = {};
    }
    let csrfH = 'X-CSRF-Token';
    params.headers[csrfH] = csrf.value;
    return fetch(url, params).then(r => {
        if(r.headers.get(csrfH)) {
            csrf.value = r.headers.get(csrfH);
        }
        callback(r);
    });
}

init();
