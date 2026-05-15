/**
 * ODK Google Drive Uploader
 * 
 * İKİ MOD:
 *   1. WEBHOOK (POST) — tek URL indirir, anında döner
 *   2. QUEUE (timer) — Drive'daki JSON dosyasını okuyup sırayla indirir
 *
 * Ayarlar (Script Properties):
 *   QUEUE_FILE_ID  — Drive'daki JSON queue dosyasının ID'si (queue modu için)
 *   GDRIVE_FOLDER  — İndirilenlerin kaydedileceği Drive klasör ID'si (opsiyonel)
 *   DAILY_LIMIT    — Günlük max indirme (varsayılan: 100)
 *   DONE_SET       — İşlenmiş URL'ler (internal, otomatik)
 *
 * Webhook deployment:
 *   Deploy → New deployment → Web App → Execute as: Me → Access: Anyone
 *   URL'yi --gdrive-url flag'inde kullan
 *   POST body: {"url":"...", "name":"...", "folderId":"..."}
 *   
 * Queue kurulumu:
 *   1. ODK'dan export et: ./odk list --export queue.json
 *   2. queue.json'u Drive'a yükle, file ID'sini al
 *   3. Script Properties'e QUEUE_FILE_ID = o ID
 *   4. Triggers → Add Trigger → processQueue → Time-driven (her 30dk)
 */

var DEFAULT_LIMIT = 100;

// ─── WEBHOOK MODU ───────────────────────────────────────────────

function doPost(e) {
  var props = PropertiesService.getScriptProperties();
  var today = getToday();
  var limit = parseInt(props.getProperty('DAILY_LIMIT') || String(DEFAULT_LIMIT));
  var count = parseInt(props.getProperty('COUNT_' + today) || '0');

  if (count >= limit) {
    return respond(429, { error: 'Daily limit reached', limit: limit, today: today });
  }

  var data;
  try {
    data = JSON.parse(e.postData.contents);
  } catch (ex) {
    return respond(400, { error: 'Invalid JSON' });
  }

  if (!data.url) {
    return respond(400, { error: 'Missing url' });
  }

  var result = downloadFile(data);

  if (!result.error) {
    count++;
    props.setProperty('COUNT_' + today, String(count));
  }

  result.remaining = Math.max(0, limit - count);
  return respond(result.error ? 500 : 200, result);
}

function doGet() {
  return respond(200, { status: 'ODK GDrive Uploader', mode: 'webhook+queue' });
}

function respond(code, data) {
  return ContentService
    .createTextOutput(JSON.stringify(data))
    .setMimeType(ContentService.MimeType.JSON);
}

// ─── QUEUE MODU (timer trigger) ────────────────────────────────

function processQueue() {
  var queueFileId = PropertiesService.getScriptProperties().getProperty('QUEUE_FILE_ID');
  if (!queueFileId) {
    Logger.log('QUEUE_FILE_ID not set — nothing to process');
    return;
  }

  var files = readQueue(queueFileId);
  if (files.length === 0) {
    Logger.log('Queue is empty');
    return;
  }

  var done = getDoneSet();
  var limit = getDailyLimit();
  var today = getToday();
  var count = parseInt(PropertiesService.getScriptProperties().getProperty('COUNT_' + today) || '0');
  var results = [];

  for (var i = 0; i < files.length && count < limit; i++) {
    var item = files[i];
    var key = item.url || item.URL;

    if (!key) continue;
    if (done[key]) continue;

    Logger.log('Downloading: ' + key);
    var result = downloadFile({ url: key, name: item.name || item.Name, folderId: item.folderId });

    if (result.error) {
      Logger.log('Failed: ' + result.error);
    } else {
      count++;
      done[key] = true;
    }
    results.push(result);
  }

  PropertiesService.getScriptProperties().setProperty('COUNT_' + today, String(count));
  saveDoneSet(done);
  updateQueue(queueFileId, files, done);

  Logger.log('Processed ' + results.length + ' files (' + count + '/' + limit + ' today)');
  return results;
}

function readQueue(fileId) {
  try {
    var file = DriveApp.getFileById(fileId);
    var json = file.getBlob().getDataAsString();
    var data = JSON.parse(json);

    // desteklenen formatlar:
    // [{"url":"...","name":"...","size":123,"cat":"..."}]
    // [{"URL":"...","Name":"...","Size":123,"Category":"..."}]
    // FileEntry formatı da çalışır (URL, Name, Size, Category alanları)
    return Array.isArray(data) ? data : [];
  } catch (e) {
    Logger.log('readQueue error: ' + e);
    return [];
  }
}

function updateQueue(fileId, files, doneSet) {
  var remaining = files.filter(function(f) { return !doneSet[f.url || f.URL]; });
  try {
    var file = DriveApp.getFileById(fileId);
    file.setContent(JSON.stringify(remaining, null, 2));
  } catch (e) {
    Logger.log('updateQueue error: ' + e);
  }
}

// ─── ORTAK ──────────────────────────────────────────────────────

function downloadFile(data) {
  try {
    var url = data.url;
    var name = data.name || url.split('/').pop().split('?')[0] || 'unknown';
    var folderId = data.folderId || PropertiesService.getScriptProperties().getProperty('GDRIVE_FOLDER');

    var response = UrlFetchApp.fetch(url, {
      muteHttpExceptions: true,
      followRedirects: true,
      timeout: 300
    });

    if (response.getResponseCode() >= 400) {
      return { url: url, error: 'HTTP ' + response.getResponseCode() };
    }

    var blob = response.getBlob();
    var folder = folderId ? DriveApp.getFolderById(folderId) : DriveApp.getRootFolder();
    var file = folder.createFile(blob.setName(name));

    return {
      url: url,
      driveId: file.getId(),
      size: file.getSize(),
      mime: file.getMimeType(),
      name: file.getName()
    };
  } catch (e) {
    return { url: data.url, error: e.toString() };
  }
}

function getDoneSet() {
  var json = PropertiesService.getScriptProperties().getProperty('DONE_SET');
  return json ? JSON.parse(json) : {};
}

function saveDoneSet(set) {
  PropertiesService.getScriptProperties().setProperty('DONE_SET', JSON.stringify(set));
}

function getDailyLimit() {
  var limit = PropertiesService.getScriptProperties().getProperty('DAILY_LIMIT');
  return limit ? parseInt(limit) : DEFAULT_LIMIT;
}

function getToday() {
  return Utilities.formatDate(new Date(), Session.getScriptTimeZone(), 'yyyy-MM-dd');
}

// ─── YÖNETİM FONKSİYONLARI ────────────────────────────────────

function setQueueFile(id) {
  PropertiesService.getScriptProperties().setProperty('QUEUE_FILE_ID', id);
}

function setFolder(id) {
  PropertiesService.getScriptProperties().setProperty('GDRIVE_FOLDER', id);
}

function setDailyLimit(limit) {
  PropertiesService.getScriptProperties().setProperty('DAILY_LIMIT', String(limit));
}

function resetToday() {
  var today = getToday();
  PropertiesService.getScriptProperties().deleteProperty('COUNT_' + today);
}

function resetAll() {
  var props = PropertiesService.getScriptProperties();
  props.deleteProperty('DONE_SET');
  // COUNT_ prefix'li tüm key'leri temizle
  var keys = props.getKeys();
  for (var i = 0; i < keys.length; i++) {
    if (keys[i].indexOf('COUNT_') === 0) {
      props.deleteProperty(keys[i]);
    }
  }
}

function status() {
  var today = getToday();
  var count = parseInt(PropertiesService.getScriptProperties().getProperty('COUNT_' + today) || '0');
  var limit = getDailyLimit();
  var done = getDoneSet();
  var qFile = PropertiesService.getScriptProperties().getProperty('QUEUE_FILE_ID');
  var qFolder = PropertiesService.getScriptProperties().getProperty('GDRIVE_FOLDER');

  var info = {
    today: today,
    processed: count,
    limit: limit,
    remaining: Math.max(0, limit - count),
    uniqueFiles: Object.keys(done).length,
    queueFileId: qFile,
    targetFolder: qFolder
  };

  Logger.log(JSON.stringify(info, null, 2));
  return info;
}
