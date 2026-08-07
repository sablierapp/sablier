// Increment a counter in the page title every second so users can tell
// the page is alive even while waiting for the meta-refresh to fire.
(function () {
  'use strict';

  var base = document.title.replace(/^\(\d+s\)\s*/, '') || 'Starting up\u2026';
  var elapsed = 0;

  setInterval(function () {
    elapsed += 1;
    document.title = '(' + elapsed + 's) ' + base;
  }, 1000);
}());
