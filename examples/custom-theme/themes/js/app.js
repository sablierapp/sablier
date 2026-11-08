// Keep the page alive between two meta-refreshes: a counter in the tab title
// and, when Sablier knows them, the startup timing rendered on the server.
(function () {
  'use strict';

  var base = document.title.replace(/^\(\d+s\)\s*/, '') || 'Starting up\u2026';
  var timing = document.getElementById('timing');
  var text = document.getElementById('timing-text');
  var bar = document.getElementById('timing-bar');
  // How long the start has lasted, on the server clock, when the page was
  // rendered; without it, count from the page load as before.
  var elapsed = timing ? parseFloat(timing.dataset.elapsed) : 0;
  var typical = timing ? parseFloat(timing.dataset.typical) : 0;
  var loaded = Date.now();

  function format(seconds) {
    seconds = Math.round(seconds);
    return seconds < 60 ? seconds + ' s' : Math.floor(seconds / 60) + ' min ' + (seconds % 60) + ' s';
  }

  function update() {
    var e = elapsed + (Date.now() - loaded) / 1000;
    document.title = '(' + Math.round(e) + 's) ' + base;
    if (!text) {
      return;
    }
    if (typical > 0 && e > 2 * typical) {
      timing.classList.add('late');
      text.textContent = 'Started ' + format(e) + ' ago. This start is taking longer than usual (about ' + format(typical) + ').';
    } else if (typical > 0) {
      text.textContent = 'Started ' + format(e) + ' ago, usually ready in about ' + format(typical) + '.';
    } else {
      text.textContent = 'Started ' + format(e) + ' ago.';
    }
    if (bar) {
      // Never full before the instance is ready.
      bar.style.width = Math.min(e / typical, 0.95) * 100 + '%';
    }
  }

  update();
  setInterval(update, 1000);
}());
