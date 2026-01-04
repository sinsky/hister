{{define "main"}}
<div class="search">
    <input type="text" autofocus placeholder="Search..." id="search" />
</div>
<div class="container">
    <div id="results-header" class="hidden">
        <div>Total number of results: <span class="results-num"></span></div>
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
<input type="hidden" id="ws-url" value="{{ .Config.WebSocketURL }}" />
<input type="hidden" id="search-url" value="{{ .Config.App.SearchURL }}" />
<script src="static/js/search.js"></script>
{{ end }}
