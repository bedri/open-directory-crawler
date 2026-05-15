/**
 * ODK Google Drive Queue Processor
 * 
 * Kurulum:
 *   1. Extensions → Apps Script → bu kodu yapıştır
 *   2. Deploy → New deployment → Web App
 *   3. Script Properties'e QUEUE_FILE_ID ekle (Drive'daki JSON dosyasının ID'si)
 *   4. Time-driven trigger kur (her 30dk veya saatte bir)
 * 
 * Queue dosyası formatı (Drive'da bir JSON dosyası):
 *   [{"url":"...", "name":"...", "size":123, "cat":"video"}]
 * 
 * Ayarlar (Script Properties):
 *   QUEUE_FILE_ID  — Drive'daki JSON queue dosyasının ID'si
 *   GDRIVE_FOLDER  — İndirilenlerin kaydedileceği Drive klasör ID'si (opsiyonel)
 *   DAILY_LIMIT    — Günlük max indirme (varsayılan: 100)
 */

var DEFAULT_LIMIT = 100;

function processQueue() {
  var queueFileId = PropertiesService.getScriptProperties().getProperty('QUEUE_FILE_ID');
  if (!queueFileId) {
    Logger.log('QUEUE_FILE_ID not set');
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
    var key = item.url;

    if (done[key]) {
      continue;
    }

    Logger.log('Downloading: ' + item.url);
    var result = downloadFile(item);

    if (result.error) {
      Logger.log('Failed: ' + result.error);
    } else {
      count++;
      done[key] = true;
    }
    results.push(result);
  }

  PropertiesService.getScriptProperties().setProperty('COUNT_' + today, count.toString());
  saveDoneSet(done);
  updateQueue(queueFileId, files, done);

  Logger.log('Processed ' + results.length + ' files (' + count + '/' + limit + ' today)');
  return results;
}

function readQueue(fileId) {
  try {
    var file = DriveApp.getFileById(fileId);
    var blob = file.getBlob();
    var json = blob.getDataAsString();
    var data = JSON.parse(json);
    return Array.isArray(data) ? data : [];
  } catch (e) {
    Logger.log('readQueue error: ' + e);
    return [];
  }
}

function updateQueue(fileId, files, doneSet) {
  var remaining = files.filter(function(f) { return !doneSet[f.url]; });
  try {
    var file = DriveApp.getFileById(fileId);
    file.setContent(JSON.stringify(remaining, null, 2));
  } catch (e) {
    Logger.log('updateQueue error: ' + e);
  }
}

function downloadFile(item) {
  try {
    var url = item.url;
    var name = item.name || url.split('/').pop().split('?')[0] || 'unknown';
    var folderId = PropertiesService.getScriptProperties().getProperty('GDRIVE_FOLDER');

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
    return { url: item.url, error: e.toString() };
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

function setQueueFile(id) {
  PropertiesService.getScriptProperties().setProperty('QUEUE_FILE_ID', id);
}

function setFolder(id) {
  PropertiesService.getScriptProperties().setProperty('GDRIVE_FOLDER', id);
}

function setDailyLimit(limit) {
  PropertiesService.getScriptProperties().setProperty('DAILY_LIMIT', limit.toString());
}

function resetToday() {
  var today = getToday();
  PropertiesService.getScriptProperties().deleteProperty('COUNT_' + today);
}

function resetAll() {
  PropertiesService.getScriptProperties().deleteProperty('DONE_SET');
  PropertiesService.getScriptProperties().deleteProperty('COUNT_');
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
