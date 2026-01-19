import {
    Document,
    extractData,
} from '../modules/extract';

let d : Document;
// ms
const defaultSleepTime = 10*1000;
let sleepTime = defaultSleepTime;
const sleepIncrementRatio = 2;

window.addEventListener("load", extract, false);

function extract() {
    try {
        d = extractData();
    } catch(e) {
        console.log("failed to extract page data:", e);
        return;
    }
    chrome.runtime.sendMessage({data:  d}, resp => {});
    setTimeout(update, sleepTime);
}


function update() {
    let d2;
    try {
        d2 = extractData();
    } catch(e) {
        console.log("failed to extract page data", e);
        return;
    }
    if(d2.html != d.html) {
        sleepTime = defaultSleepTime;
        d = d2;
        chrome.runtime.sendMessage({data:  d}, resp => {});
    } else {
        sleepTime *= sleepIncrementRatio;
    }
    setTimeout(update, sleepTime);
}

// Get message from background page
// TODO check sender
chrome.runtime.onMessage.addListener(function(request, sender, sendResponse) {
    if(!request) {
        return;
    }
    if(request.error) {
        alert(request.error);
        return;
    }
    if(request.action == "reindex") {
        extract();
        sendResponse({"action": "reindex", "status": "ok"});
		return;
    }
    console.log("message received", request)
});
