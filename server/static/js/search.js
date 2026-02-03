let ws;
let input = document.getElementById("search");
let autocomplete = document.getElementById("autocomplete");
let results = document.getElementById("results");
let emptyImg = "data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw==";
let urlState = {};
let templates = {};
for(let el of document.querySelectorAll("template")) {
    let id = el.getAttribute("id")
    templates[id] = el;
}

const tips = [
    'Use <code>*</code> for partial match.<br />Prefixing word with <code>-</code> excludes matching documents.',
    'Click on the three dots near the result URL to specify priority queries for that result.',
    'Press <code>enter</code> to open the first result.',
    'Use <code>alt+k</code> and <code>alt+j</code> to navigate between results.',
    'Press <code>alt+o</code> to open current search query in your configured search engine.',
    'Use <code>url:</code> prefix to search only in the URL field. E.g.: <code>url:*github* hister</code>.',
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
        setTimeout(connect, 1000); // Reconnect after 1 second
    };

    ws.onerror = function(error) {
        console.error("WebSocket error:", error);
    };
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
        document.querySelector(".result").classList.add("highlight");
    }
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
        autocomplete.value = "Search...";
        return;
    }
    if(!suggestion) {
        autocomplete.value = "";
        return;
    }
    autocomplete.value = suggestion.replaceAt(0, input.value);
}

function openUrl(u, newWindow) {
    if(newWindow) {
        window.open(u, '_blank');
        window.focus();
        return;
    }
    window.location.href = u;
}

function init() {
    results.replaceChildren(createTips());
    connect();
}

function openResult(e, newWindow) {
    if(e.preventDefault) {
        e.preventDefault();
    }
    let link = e.target.closest('.result-title a')
    let url = link.getAttribute("href");
    let title = link.innerText;
    if(!link.classList.contains("error")) {
        saveHistoryItem(url, title, input.value).then((r) => {
            openUrl(url, newWindow);
        });
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
    });
    if(res.query && res.query.text != input.value) {
        header.querySelector(".expanded-query").innerHTML = `Expanded query: <code>"${res.query.text}"</code>`;
    }
    return header;
}

function createPriorityResult(r) {
    let rn = createTemplate("result", {
        "a": (e) => {
            e.setAttribute("href", r.url);
            e.innerHTML = r.title || "*title*";
            // TODO handle middleclick (auxclick handler)
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
            // TODO handle middleclick (auxclick handler)
            clickHandler(e, openResult, ev => openResult(ev, true));
        },
        ".readable": e => createReadable(e, r.url),
        "img": e => e.setAttribute("src", r.favicon || emptyImg),
        ".result-url": e => e.textContent = r.url,
        ".action-button": e => e.addEventListener("click", (ev) => toggleActions(ev, e.closest(".result"))),
        "p": e => e.innerHTML = r.text || "",
    });
    return rn;
}

function clickHandler(e, leftClickCallback, middleClickCallback) {
    e.addEventListener("click", leftClickCallback);
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
    fetch(e.target.getAttribute("data-href")).then(resp => resp.text()).then(t => openPopup(h, t));
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
        ".delete": (e) => e.addEventListener("click", () => savePriorityResult(e, true)),
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
        ".save": (e) => e.addEventListener("click", () => savePriorityResult(e, false)),
        ".close": (e) => e.addEventListener("click", () => closeActions(e)),
    });
    for(let e of a.children) {
        e.style.animation = "fade-in 0.5s";
    }
    res.appendChild(a);
}

function closeActions(e) {
    e.closest(".actions").remove();
}

function savePriorityResult(e, remove) {
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
    saveHistoryItem(url, title, query, remove).then((r) => {
        result.querySelector(".actions").appendChild(createTemplate("success", {
            ".message": (e) => e.innerText = `Priority result ${remove ? "deleted" : "added"}.`,
        }));
    });
}

function saveHistoryItem(url, title, query, remove) {
    return fetch("/history", {
        method: "POST",
        body: JSON.stringify({"url": url, "title": title, "query": query, "delete": remove}),
        headers: {"Content-type": "application/json; charset=UTF-8"},
    })
}

let highlightIdx = 0;
window.addEventListener("keydown", function(e) {
    if(e.key == "/") {
        if(document.activeElement != input) {
            e.preventDefault();
            input.focus();
            return;
        }
    }
    if(e.key == "Enter") {
        let newWindow = e.altKey ? true : false;
        if(input.value.startsWith("!!")) {
            openUrl(getSearchUrl(input.value.substring(2)), newWindow);
            return;
        }
        e.preventDefault();
        let res = document.querySelectorAll(".result .result-title a")[highlightIdx];
        openResult({'target': res}, newWindow);
        return
    }
    if(e.key == "Tab") {
        if(document.activeElement == input && input.value != autocomplete.value) {
            input.value = autocomplete.value;
            handleInput();
            e.preventDefault();
            return;
        }
    }
    if(e.altKey && (e.key == "j" || e.key == "k")) {
        e.preventDefault();

        let res = document.querySelectorAll(".result");
        if(res.length > highlightIdx) {
            res[highlightIdx].classList.remove("highlight");
        }
        highlightIdx = (highlightIdx+(e.key=="j"?1:-1)+res.length) % res.length;
        res[highlightIdx].classList.add("highlight");
        return;
    }
    if(e.altKey && e.key == "o") {
        e.preventDefault();
        openUrl(getSearchUrl(input.value));
        return
    }
    if(e.altKey && e.key == "v") {
        e.preventDefault();
        if(!closePopup()) {
            openReadable({'target': document.querySelectorAll(".result .readable")[highlightIdx]});
        }
        return
    }
    if(e.key == 'Escape') {
        if(closePopup()) {
            e.preventDefault();
            return;
        }
    }
});

String.prototype.replaceAt = function(index, replacement) {
    return this.substring(0, index) + replacement + this.substring(index + replacement.length);
}

init();
