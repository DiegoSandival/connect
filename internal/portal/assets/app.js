const petEmoji = document.getElementById('petEmoji');
const petName = document.getElementById('petName');
const changeAnimalBtn = document.getElementById('changeAnimalBtn');
const selectFileBtn = document.getElementById('selectFileBtn');
const fileInput = document.getElementById('fileInput');
const progressShell = document.getElementById('progressShell');
const progressBar = document.getElementById('progressBar');
const uploadList = document.getElementById('uploadList');
const emptyUploads = document.getElementById('emptyUploads');
const deleteModal = document.getElementById('deleteModal');
const deleteModalMessage = document.getElementById('deleteModalMessage');
const cancelDeleteBtn = document.getElementById('cancelDeleteBtn');
const confirmDeleteBtn = document.getElementById('confirmDeleteBtn');

let pendingDeleteName = '';

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

function resetProgress() {
  progressBar.style.width = '0%';
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
    title.textContent = item.stored_name || item.original_name || 'archivo';

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

function uploadSelectedFile(selectedFile) {
  if (!selectedFile) {
    return;
  }

  selectFileBtn.disabled = true;
  selectFileBtn.textContent = 'Subiendo...';
  progressShell.hidden = false;
  resetProgress();

  const formData = new FormData();
  formData.append('file', selectedFile);

  const request = new XMLHttpRequest();
  request.open('POST', '/api/upload');

  request.upload.addEventListener('progress', (progressEvent) => {
    if (!progressEvent.lengthComputable) {
      return;
    }
    const percent = Math.round((progressEvent.loaded / progressEvent.total) * 100);
    progressBar.style.width = `${percent}%`;
  });

  request.addEventListener('load', async () => {
    let payload = {};
    try {
      payload = JSON.parse(request.responseText || '{}');
    } catch (_) {
      payload = {};
    }

    if (request.status >= 200 && request.status < 300) {
      progressBar.style.width = '100%';
      fileInput.value = '';
      await fetchUploads();
      return;
    }

    progressShell.hidden = true;
  });

  request.addEventListener('error', () => {
    progressShell.hidden = true;
  });

  request.addEventListener('loadend', () => {
    selectFileBtn.disabled = false;
    selectFileBtn.textContent = 'Seleccionar archivo';
    window.setTimeout(() => {
      progressShell.hidden = true;
      resetProgress();
    }, 600);
  });

  request.send(formData);
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
fileInput.addEventListener('change', () => uploadSelectedFile(fileInput.files[0]));
fetchClient();
