const petEmoji = document.getElementById('petEmoji');
const petName = document.getElementById('petName');
const changeAnimalBtn = document.getElementById('changeAnimalBtn');
const selectFileBtn = document.getElementById('selectFileBtn');
const fileInput = document.getElementById('fileInput');
const progressShell = document.getElementById('progressShell');
const progressTitle = document.getElementById('progressTitle');
const progressMeta = document.getElementById('progressMeta');
const progressBar = document.getElementById('progressBar');
const progressList = document.getElementById('progressList');
const uploadList = document.getElementById('uploadList');
const emptyUploads = document.getElementById('emptyUploads');
const deleteModal = document.getElementById('deleteModal');
const deleteModalMessage = document.getElementById('deleteModalMessage');
const cancelDeleteBtn = document.getElementById('cancelDeleteBtn');
const confirmDeleteBtn = document.getElementById('confirmDeleteBtn');

let pendingDeleteName = '';
let uploadInProgress = false;

function paintIdentity(snapshot) {
  petEmoji.textContent = snapshot?.animal_emoji || '🐾';
  petName.textContent = snapshot?.identifier || snapshot?.animal_name || 'Tu animal';
}

async function fetchClient() {
  try {
    const response = await fetch('/api/client', { cache: 'no-store' });
    const snapshot = await response.json();
    paintIdentity(snapshot);
    await fetchUploads();
  } catch (error) {
    petEmoji.textContent = '🐾';
    petName.textContent = 'No se pudo cargar';
  }
}

async function changeAnimal() {
  changeAnimalBtn.disabled = true;
  changeAnimalBtn.textContent = 'Cambiando...';

  try {
    const response = await fetch('/api/animal/change', {
      method: 'POST',
      cache: 'no-store',
    });
    const snapshot = await response.json();

    if (!response.ok) {
      throw new Error(snapshot?.error || 'No se pudo cambiar el animal');
    }

    paintIdentity(snapshot);
    await fetchUploads();
  } catch (error) {
    changeAnimalBtn.textContent = 'Reintentar animal';
    changeAnimalBtn.disabled = false;
    return;
  }

  changeAnimalBtn.disabled = false;
  changeAnimalBtn.textContent = 'Cambiar animal';
}

async function activateAccess() {
  if (unlockInProgress) {
    return;
  }

  unlockInProgress = true;
  unlockButton.disabled = true;
  unlockButton.textContent = 'Activando...';

  try {
    const response = await fetch('/api/access', {
      method: 'POST',
      cache: 'no-store',
    });
    const payload = await response.json();

    if (!response.ok) {
      throw new Error(payload?.error || 'No se pudo activar internet temporal');
    }

    unlockInProgress = false;
    paintIdentity(payload);
    paintNetwork(payload);
    return;
  } catch (error) {
    unlockInProgress = false;
    await fetchAd();
    unlockHint.textContent = error.message || 'No se pudo activar internet temporal.';
    return;
  }
}

function resetProgress() {
  progressBar.style.width = '0%';
}

function formatFileSize(size) {
  if (!Number.isFinite(size) || size <= 0) {
    return '0 KB';
  }
  if (size < 1024 * 1024) {
    return `${Math.max(1, Math.round(size / 1024))} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function createProgressEntry(file) {
  const item = document.createElement('li');
  item.className = 'progress-item';

  const topRow = document.createElement('div');
  topRow.className = 'progress-item-head';

  const name = document.createElement('strong');
  name.className = 'progress-item-name';
  name.textContent = file.name;

  const state = document.createElement('span');
  state.className = 'progress-item-state';
  state.textContent = 'En cola';

  topRow.append(name, state);

  const detailRow = document.createElement('div');
  detailRow.className = 'progress-item-meta';
  detailRow.textContent = formatFileSize(file.size);

  const bar = document.createElement('div');
  bar.className = 'progress-bar progress-item-bar';

  const fill = document.createElement('div');
  fill.className = 'progress-bar-fill';
  bar.append(fill);

  item.append(topRow, detailRow, bar);
  progressList.append(item);

  return {
    file,
    item,
    state,
    detailRow,
    fill,
    progress: 0,
    status: 'queued',
  };
}

function setEntryProgress(entry, percent, detailText) {
  entry.progress = Math.max(0, Math.min(100, percent));
  entry.fill.style.width = `${entry.progress}%`;
  if (detailText) {
    entry.detailRow.textContent = detailText;
  }
}

function setEntryStatus(entry, status, stateText, detailText) {
  entry.status = status;
  entry.item.dataset.status = status;
  entry.state.textContent = stateText;
  if (detailText) {
    entry.detailRow.textContent = detailText;
  }
}

function updateQueueSummary(entries) {
  const total = entries.length;
  const completed = entries.filter((entry) => entry.status === 'done').length;
  const failed = entries.filter((entry) => entry.status === 'error').length;
  const uploading = entries.find((entry) => entry.status === 'uploading');
  const processed = completed + failed;
  const remaining = total - processed;
  const overallProgress = total
    ? Math.round(((processed + ((uploading?.progress || 0) / 100)) / total) * 100)
    : 0;

  progressBar.style.width = `${overallProgress}%`;

  if (processed === total) {
    progressTitle.textContent = failed ? 'Carga completada con errores' : 'Carga completada';
  } else if (uploading) {
    progressTitle.textContent = `Subiendo ${uploading.file.name}`;
  } else {
    progressTitle.textContent = 'Preparando archivos';
  }

  const parts = [`${completed}/${total} completados`];
  if (remaining > 0) {
    parts.push(`${remaining} restantes`);
  }
  if (failed > 0) {
    parts.push(`${failed} con error`);
  }
  progressMeta.textContent = parts.join(' · ');
}

function uploadFile(file, onProgress) {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append('file', file);

    const request = new XMLHttpRequest();
    request.open('POST', '/api/upload');

    request.upload.addEventListener('progress', (progressEvent) => {
      if (!progressEvent.lengthComputable) {
        return;
      }
      const percent = Math.round((progressEvent.loaded / progressEvent.total) * 100);
      onProgress(percent);
    });

    request.addEventListener('load', () => {
      let payload = {};
      try {
        payload = JSON.parse(request.responseText || '{}');
      } catch (_) {
        payload = {};
      }

      if (request.status >= 200 && request.status < 300) {
        onProgress(100);
        resolve(payload);
        return;
      }

      reject(new Error(payload?.error || 'No se pudo subir el archivo'));
    });

    request.addEventListener('error', () => {
      reject(new Error('No se pudo subir el archivo'));
    });

    request.send(formData);
  });
}

function renderUploads(items) {
  uploadList.innerHTML = '';

  if (!items.length) {
    emptyUploads.hidden = false;
    return;
  }

  emptyUploads.hidden = true;
  for (const item of items) {
    const li = document.createElement('li');
    li.className = 'upload-item';

    const row = document.createElement('div');
    row.className = 'upload-row';

    const title = document.createElement('strong');
    title.className = 'upload-name';
    title.textContent = item.original_name || item.stored_name || 'archivo';

    const deleteButton = document.createElement('button');
    deleteButton.className = 'delete-upload-button';
    deleteButton.type = 'button';
    deleteButton.dataset.fileName = item.stored_name || item.original_name || '';
    deleteButton.textContent = 'Eliminar';

    row.append(title, deleteButton);
    li.append(row);
    uploadList.appendChild(li);
  }
}

function openDeleteModal(fileName) {
  pendingDeleteName = fileName;
  deleteModalMessage.textContent = `Se eliminara ${fileName} y tambien se borrara de la base de datos de la empresa. Esta accion no se puede deshacer.`;
  deleteModal.hidden = false;
  confirmDeleteBtn.disabled = false;
  confirmDeleteBtn.textContent = 'Eliminar archivo';
}

function closeDeleteModal() {
  pendingDeleteName = '';
  deleteModal.hidden = true;
  confirmDeleteBtn.disabled = false;
  confirmDeleteBtn.textContent = 'Eliminar archivo';
}

async function deleteUpload(fileName) {
  confirmDeleteBtn.disabled = true;
  confirmDeleteBtn.textContent = 'Eliminando...';

  try {
    const response = await fetch('/api/upload/delete', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ name: fileName }),
    });
    const payload = await response.json();

    if (!response.ok) {
      throw new Error(payload?.error || 'No se pudo eliminar el archivo');
    }

    closeDeleteModal();
    await fetchUploads();
  } catch (error) {
    confirmDeleteBtn.disabled = false;
    confirmDeleteBtn.textContent = 'Reintentar eliminar';
  }
}

async function fetchUploads() {
  try {
    const response = await fetch('/api/uploads', { cache: 'no-store' });
    const payload = await response.json();
    renderUploads(Array.isArray(payload) ? payload : []);
  } catch (error) {
    renderUploads([]);
  }
}

async function uploadSelectedFiles(selectedFiles) {
  const files = Array.from(selectedFiles || []);
  if (!files.length || uploadInProgress) {
    return;
  }

  uploadInProgress = true;
  selectFileBtn.disabled = true;
  selectFileBtn.textContent = files.length > 1 ? `Subiendo ${files.length} archivos...` : 'Subiendo archivo...';
  progressShell.hidden = false;
  progressList.innerHTML = '';
  resetProgress();
  const entries = files.map((file) => createProgressEntry(file));
  updateQueueSummary(entries);

  let hasSuccess = false;

  for (const entry of entries) {
    setEntryStatus(entry, 'uploading', 'Subiendo...', `${formatFileSize(entry.file.size)} · 0%`);
    updateQueueSummary(entries);

    try {
      await uploadFile(entry.file, (percent) => {
        setEntryProgress(entry, percent, `${formatFileSize(entry.file.size)} · ${percent}%`);
        updateQueueSummary(entries);
      });
      setEntryStatus(entry, 'done', 'Completado', `${formatFileSize(entry.file.size)} · 100%`);
      setEntryProgress(entry, 100, `${formatFileSize(entry.file.size)} · 100%`);
      hasSuccess = true;
      window.setTimeout(() => {
        entry.item.remove();
      }, 180);
    } catch (error) {
      setEntryStatus(entry, 'error', 'Error', error.message || 'No se pudo subir el archivo');
      setEntryProgress(entry, 100, error.message || 'No se pudo subir el archivo');
    }

    updateQueueSummary(entries);
  }

  if (hasSuccess) {
    await fetchUploads();
  }

  fileInput.value = '';
  selectFileBtn.disabled = false;
  selectFileBtn.textContent = 'Seleccionar archivos';
  uploadInProgress = false;
}

selectFileBtn.addEventListener('click', () => fileInput.click());
changeAnimalBtn.addEventListener('click', () => {
  void changeAnimal();
});
uploadList.addEventListener('click', (event) => {
  const deleteButton = event.target.closest('.delete-upload-button');
  if (!deleteButton) {
    return;
  }

  const { fileName } = deleteButton.dataset;
  if (!fileName) {
    return;
  }

  openDeleteModal(fileName);
});
cancelDeleteBtn.addEventListener('click', closeDeleteModal);
confirmDeleteBtn.addEventListener('click', () => {
  if (!pendingDeleteName) {
    return;
  }
  void deleteUpload(pendingDeleteName);
});
deleteModal.addEventListener('click', (event) => {
  if (event.target === deleteModal) {
    closeDeleteModal();
  }
});
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && !deleteModal.hidden) {
    closeDeleteModal();
  }
});
fileInput.addEventListener('change', () => uploadSelectedFiles(fileInput.files));
fetchClient();
