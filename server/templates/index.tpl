{{ define "main" }}
<div class="search text-center">
    <input type="text" autofocus placeholder="Search..." id="search" />
</div>
<div class="container">
    <div id="results-header" class="hidden">
        <div>Total number of results: <b class="results-num"></b></div>
        <div class="expanded-query"></div>
    </div>
    <div id="results"></div>
</div>
<template id="result">
    <div class="result">
        <div class="result-title"><img><a></a></div>
        <span class="result-url"></span>
        <p class="result-content"></p>
    </div>
</template>
<template id="tips">
<div class="text-center">
    <b>Tips</b><br />
    Use <code>*</code> for partial match.<br />
    Prefixing word with <code>+</code> makes it mandatory.<br />
    Prefixing word with <code>-</code> excludes matching documents.
</div>
</template>
<input type="hidden" id="ws-url" value="{{ .Config.WebSocketURL }}" />
<input type="hidden" id="search-url" value="{{ .Config.App.SearchURL }}" />
<script src="static/js/search.js"></script>
{{ end }}
