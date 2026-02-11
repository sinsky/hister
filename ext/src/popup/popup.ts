const urlInput = <HTMLInputElement>document.querySelector("#url");
const msgBox = <HTMLElement>document.querySelector("#msg");

const defaultURL = "http://127.0.0.1:4433/";

chrome.runtime.onMessage.addListener(function(request, sender, sendResponse) {
    if(!request) {
        return;
    }
});

function saveURL() {
    let u = urlInput.value;
	chrome.storage.local.set({
		histerURL: u,
	}).then(() => {
        msgBox.innerText = "Settings saved";
    });
}

document.querySelector("form").addEventListener("submit", (e) => {
    saveURL();
    e.preventDefault();
});

chrome.storage.local.get(['histerURL'], (d) => {
    if(!d['histerURL']) {
        chrome.storage.local.set({
            histerURL: defaultURL,
        });
    }
    urlInput.setAttribute('value', d['histerURL'] || defaultURL);
});

document.querySelector("#reindex").addEventListener("click", (e) => {
	chrome.tabs.query({active: true, currentWindow: true}, function(tabs) {
		if(!tabs) return;
		chrome.tabs.sendMessage(tabs[0].id, {action: "reindex"}, (r) => {
            if(r && r.status == "ok") {
                msgBox.innerText = "Reindex successful";
                return;
            }
            msgBox.innerText = "Reindex failed";
            if(r && r.error) {
                msgBox.innerText += ": " + r.error;
            }
        });
	});
});

