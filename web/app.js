(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };
  var STATES = ["idle", "uploading", "done", "error"];
  var MAX = 100 * 1024 * 1024;
  var MAX_LABEL = "100MB";
  var RETENTION_LABEL = "30日";

  function humanSize(b) {
    if (b >= 1073741824) return (b / 1073741824).toFixed(0) + "GB";
    if (b >= 1048576) return (b / 1048576).toFixed(0) + "MB";
    if (b >= 1024) return (b / 1024).toFixed(0) + "KB";
    return b + "B";
  }
  function humanRetention(hours) {
    if (hours % 24 === 0) return (hours / 24) + "日";
    return "約" + Math.round(hours / 24) + "日";
  }

  function loadConfig() {
    fetch("/api/config").then(function (r) { return r.json(); }).then(function (c) {
      if (c && typeof c.maxUploadBytes === "number") {
        MAX = c.maxUploadBytes;
        MAX_LABEL = humanSize(MAX);
      }
      if (c && typeof c.retentionHours === "number") {
        RETENTION_LABEL = humanRetention(c.retentionHours);
      }
      $("hint").textContent = "最大 " + MAX_LABEL + " ・ " + RETENTION_LABEL + "後に自動削除";
      $("done-note").textContent = "※アップロードしたファイルは" + RETENTION_LABEL + "後に自動で削除されます";
    }).catch(function () { /* keep defaults */ });
  }
  loadConfig();

  function show(name) {
    STATES.forEach(function (s) { $(s).classList.toggle("hidden", s !== name); });
  }
  function fail(msg) { $("err-text").textContent = msg; show("error"); }
  function reset() { $("fileinput").value = ""; show("idle"); }

  function upload(file) {
    if (file.size > MAX) { fail("ファイルサイズが大きすぎます（最大" + MAX_LABEL + "）"); return; }
    show("uploading");
    $("up-filename").textContent = file.name;
    $("bar").style.width = "0%";
    $("pct").textContent = "0%";

    var xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/upload");
    xhr.setRequestHeader("X-Filename", encodeURIComponent(file.name));
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");

    xhr.upload.onprogress = function (e) {
      if (e.lengthComputable) {
        var p = Math.round((e.loaded / e.total) * 100);
        $("bar").style.width = p + "%";
        $("pct").textContent = p + "%";
      }
    };
    xhr.onload = function () {
      if (xhr.status >= 200 && xhr.status < 300) {
        var res = JSON.parse(xhr.responseText);
        var url = res.url || (location.origin + "/d/" + res.id);
        $("shareurl").value = url;
        show("done");
      } else {
        fail(xhr.status === 413
          ? "ファイルサイズが大きすぎます（最大" + MAX_LABEL + "）"
          : "アップロードに失敗しました");
      }
    };
    xhr.onerror = function () { fail("通信エラーが発生しました"); };
    xhr.send(file);
  }

  $("fileinput").addEventListener("change", function (e) {
    if (e.target.files[0]) upload(e.target.files[0]);
  });

  var dz = $("dropzone");
  ["dragenter", "dragover"].forEach(function (ev) {
    dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add("drag"); });
  });
  ["dragleave", "drop"].forEach(function (ev) {
    dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove("drag"); });
  });
  dz.addEventListener("drop", function (e) {
    var f = e.dataTransfer.files[0];
    if (f) upload(f);
  });

  $("retry").addEventListener("click", reset);
  $("again").addEventListener("click", reset);

  $("copybtn").addEventListener("click", function () {
    var input = $("shareurl");
    var btn = $("copybtn");
    var done = function () {
      btn.textContent = "コピーしました！";
      setTimeout(function () { btn.textContent = "コピー"; }, 2000);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(input.value).then(done, function () {
        input.select(); document.execCommand("copy"); done();
      });
    } else {
      input.select(); document.execCommand("copy"); done();
    }
  });
})();
