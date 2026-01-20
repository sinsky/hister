import {
    sendDocument,
} from '../modules/network';

const missingURLMsg = {"error": "Missing or invalid Hister server URL. Configure it in the addon popup."};
// TODO check source
async function cjsMsgHandler(request, sender, sendResponse) {
    let d = request.data;
    chrome.storage.local.get(['histerURL']).then(data => {
        let u = data['histerURL'] || "";
        if(!u) {
            chrome.tabs.sendMessage(sender.tab.id, missingURLMsg);
            return;
        }
        if(!u.endsWith('/')) {
            u += '/';
        }
        sendDocument(u+"add", d).then((r) => sendResponse({"msg": "ok"})).catch(err => sendResponse({"error": err}));
    }).catch(error => {
        chrome.tabs.sendMessage(sender.tab.id, missingURLMsg);
    });
}

chrome.runtime.onMessage.addListener(cjsMsgHandler);
