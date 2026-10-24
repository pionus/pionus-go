(function () {
  'use strict';

  var root = document.documentElement;

  var toggle = document.getElementById('theme-toggle');
  if (toggle) {
    var sync = function () {
      var dark = root.dataset.theme === 'dark';
      toggle.textContent = dark ? '☀' : '◐';
      toggle.setAttribute('aria-label', dark ? '切换到浅色模式' : '切换到深色模式');
    };
    sync();
    toggle.addEventListener('click', function () {
      var next = root.dataset.theme === 'dark' ? 'light' : 'dark';
      root.dataset.theme = next;
      try { localStorage.setItem('pionus-theme', next); } catch (e) {}
      sync();
    });
  }

  document.querySelectorAll('.prose pre').forEach(function (pre) {
    var code = pre.querySelector('code');
    if (!code) return;
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'copy';
    button.textContent = '复制';
    button.addEventListener('click', function () {
      var text = code.innerText;
      var done = function () {
        button.textContent = '已复制';
        setTimeout(function () { button.textContent = '复制'; }, 1500);
      };
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(done, done);
      } else {
        var area = document.createElement('textarea');
        area.value = text;
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.select();
        try { document.execCommand('copy'); } catch (e) {}
        document.body.removeChild(area);
        done();
      }
    });
    pre.appendChild(button);
  });

  function brokenImage(img) {
    if (img.dataset.broken) return;
    img.dataset.broken = '1';
    var span = document.createElement('span');
    span.className = 'img-broken';
    var link = document.createElement('a');
    link.href = img.currentSrc || img.src;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.textContent = '图片加载失败，点击查看原图';
    span.appendChild(link);
    img.replaceWith(span);
  }

  document.querySelectorAll('.prose img').forEach(function (img) {
    img.addEventListener('error', function () { brokenImage(img); });
    if (img.complete && img.naturalWidth === 0) brokenImage(img);
  });

  var tocLinks = Array.prototype.slice.call(document.querySelectorAll('.toc a'));
  if (tocLinks.length && 'IntersectionObserver' in window) {
    var byHeading = new Map();
    tocLinks.forEach(function (link) {
      var id = link.hash.slice(1);
      try { id = decodeURIComponent(id); } catch (e) {}
      var heading = document.getElementById(id);
      if (heading) byHeading.set(heading, link);
    });
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        tocLinks.forEach(function (link) { link.classList.remove('active'); });
        var link = byHeading.get(entry.target);
        if (link) link.classList.add('active');
      });
    }, { rootMargin: '-88px 0px -70% 0px' });
    byHeading.forEach(function (_link, heading) { observer.observe(heading); });
  }

  if (window.matchMedia && window.matchMedia('(max-width: 1000px)').matches) {
    document.querySelectorAll('.toc details').forEach(function (details) {
      details.removeAttribute('open');
    });
  }

  var progress = document.getElementById('progress');
  if (progress) {
    var updateProgress = function () {
      var max = document.documentElement.scrollHeight - window.innerHeight;
      var ratio = max > 0 ? window.scrollY / max : 0;
      progress.style.width = Math.max(0, Math.min(1, ratio)) * 100 + '%';
    };
    window.addEventListener('scroll', updateProgress, { passive: true });
    window.addEventListener('resize', updateProgress);
    updateProgress();
  }

  var parentField = document.getElementById('parent_id');
  var replyHint = document.getElementById('reply-hint');
  var cancelReply = document.getElementById('cancel-reply');

  document.querySelectorAll('.reply').forEach(function (button) {
    button.addEventListener('click', function () {
      if (!parentField) return;
      parentField.value = button.dataset.reply || '0';
      if (replyHint) {
        replyHint.hidden = false;
        replyHint.textContent = '正在回复 ' + (button.dataset.author || '') + '，点击“取消回复”可改为评论文章。';
      }
      if (cancelReply) cancelReply.hidden = false;
      var form = document.getElementById('comment-form');
      var textarea = form ? form.querySelector('textarea') : null;
      if (form) form.scrollIntoView({ behavior: 'smooth', block: 'center' });
      if (textarea) textarea.focus();
    });
  });

  if (cancelReply) {
    cancelReply.addEventListener('click', function () {
      if (parentField) parentField.value = '0';
      if (replyHint) { replyHint.hidden = true; replyHint.textContent = ''; }
      cancelReply.hidden = true;
    });
  }
})();
