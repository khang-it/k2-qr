// app.js
// ============== Load ngân hàng ==============
let bankArray = [];
async function loadBanks() {
    try {
        const r = await fetch("https://api.vietqr.io/v2/banks");
        const j = await r.json();
        bankArray = j.data || [];
    } catch (e) { console.error("load banks error", e); }
}

// ============== Lấy phần tử DOM ==============
const REFRESH_SECONDS = 10; // 30 giây
const searchInput = document.getElementById("bankSearch");
const bankList = document.getElementById("bankList");
const avatarPreview = document.getElementById("avatarPreview");
const qrImage = document.getElementById("qrImage");
const qrResult = document.getElementById("qrResult");
const qrExpiry = document.getElementById("qrExpiry");
const contentArea = document.getElementById("contentArea");
const emvPreview = document.getElementById("emvPreview");
const previewLink = document.getElementById("previewLink");
const copyBtn = document.getElementById("copyBtn");
const printBtn = document.getElementById("printBtn");
const qrColor = document.getElementById("qrColor");
const qrColorText = document.getElementById("qrColorText");
const avatarInput = document.getElementById("avatarInput");
const avatarBtn = document.getElementById("avatarBtn");
const previewWrapper = document.getElementById("previewWrapper");
const avatarRemoveBtn = document.getElementById("avatarRemoveBtn");


avatarInput.addEventListener("change", () => {
    if (avatarInput.files && avatarInput.files[0]) {
        const file = avatarInput.files[0];
        avatarPreview.src = URL.createObjectURL(file);
        previewWrapper.classList.remove("hidden");
    } else {
        previewWrapper.classList.add("hidden");
        avatarPreview.src = "";
    }
});

avatarRemoveBtn.addEventListener("click", () => {
    avatarInput.value = ""; // reset file input
    avatarPreview.src = "";
    previewWrapper.classList.add("hidden");
    localStorage.setItem("avatar", "");
    updateAllWithExpiry();
});

avatarBtn.addEventListener("click", () => avatarInput.click());

qrColor.addEventListener("input", () => {
    qrColorText.value = (qrColor.value || '').toUpperCase();
});

// ============== In QR ==============
printBtn?.addEventListener("click", () => {
    const imgSrc = qrImage.src;
    if (!imgSrc) return;
    const qrSize = parseInt(document.getElementById("qrSize").value) || 512;
    const iframe = document.getElementById("printFrame");
    const doc = iframe.contentDocument || iframe.contentWindow.document;
    const style = `
    <style>
      @page { size: ${qrSize}px ${qrSize}px; margin:0; }
      html, body { margin:0; padding:0; height:100%; display:flex; justify-content:center; align-items:center; }
      img { width:${qrSize}px; height:${qrSize}px; display:block; }
    </style>
  `;
    doc.open();
    doc.write(`<html><head>${style}</head><body><img src="${imgSrc}" /></body></html>`);
    doc.close();
    doc.querySelector("img").onload = () => {
        iframe.contentWindow.focus();
        iframe.contentWindow.print();
    };
});

// ============== Hàm phụ ==============
function debounce(fn, wait = 180) {
    let t; return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), wait) }
}

function selectBank(bank) {
    searchInput.value = bank.shortName + " - " + bank.name;
    bankList.classList.add("hidden");
    document.getElementById("bankInfo").classList.remove("hidden");
    document.getElementById("bankLogo").src = bank.logo;
    document.getElementById("bankName").textContent = bank.name;
    document.getElementById("bankBinShow").textContent = "BIN: " + bank.bin;
    searchInput.dataset.bin = bank.bin;
    updateAllWithExpiry();
}

function getParams() {
    return {
        bankBin: searchInput.dataset.bin || "",
        accountNo: document.getElementById("accountNo").value.trim(),
        receiverName: document.getElementById("receiverName").value.trim(),
        amount: document.getElementById("amount").value.trim(),
        desc: document.getElementById("desc").value.trim(),
        size: document.getElementById("qrSize").value || "512",
        qrcolor: document.getElementById("qrColor").value,
        timeStamp: Date.now().toString(),
        avatar: localStorage.getItem("avatar")
    };
}

async function updateContentPreview(params) {
    const u = new URL("/orbit-qr/content", location.origin);
    Object.keys(params).forEach(k => { if (params[k]) u.searchParams.set(k, params[k]); });
    const res = await fetch(u.toString());
    if (!res.ok) { emvPreview.classList.add("hidden"); return; }
    const j = await res.json();
    contentArea.value = j.content || "";
    emvPreview.classList.remove("hidden");
    const linkUrl = new URL("/orbit-qr", location.origin);
    Object.keys(params).forEach(k => { if (params[k]) linkUrl.searchParams.set(k, params[k]); });
    previewLink.value = linkUrl.toString();
}

function updateQRImage(params) {
    const formData = new FormData();
    Object.keys(params).forEach(k => { if (params[k]) formData.append(k, params[k]); });

    if (avatarInput.files[0]) {
        formData.append("avatar", avatarInput.files[0]);
    } else {
        const base64 = localStorage.getItem("avatar");
        if (base64 && base64.startsWith("data:image")) {
            const blob = base64ToBlob(base64);
            formData.append("avatar", blob, "avatar.png");
        }
    }

    fetch("/orbit-qr", { method: "POST", body: formData })
        .then(r => r.blob())
        .then(b => {
            qrImage.src = URL.createObjectURL(b);
            document.getElementById("downloadBtn").href = qrImage.src;
            qrResult.classList.remove("hidden");
            document.getElementById("downloadBtn").classList.remove("hidden");
        });
}

function base64ToBlob(base64) {
    const arr = base64.split(',');
    const mime = arr[0].match(/:(.*?);/)[1];
    const bstr = atob(arr[1]);
    let n = bstr.length;
    const u8arr = new Uint8Array(n);
    while (n--) u8arr[n] = bstr.charCodeAt(n);
    return new Blob([u8arr], { type: mime });
}


function updateAll() {
    const p = getParams();
    if (!p.bankBin || !p.accountNo || !p.receiverName) {
        qrResult.classList.add("hidden");
        emvPreview.classList.add("hidden");
        return;
    }
    updateContentPreview(p);
    updateQRImage(p);
}

// ============== Countdown & Refresh ==============
let countdownInterval = null;
function startCountdown() {
    clearInterval(countdownInterval);
    let remaining = REFRESH_SECONDS;
    function update() {
        const minutes = String(Math.floor(remaining / 60)).padStart(2, '0');
        const seconds = String(remaining % 60).padStart(2, '0');
        document.getElementById("countdown").innerText = `${minutes}:${seconds}`;
        if (remaining <= 0) {
            clearInterval(countdownInterval);
            updateAllWithExpiry();
        }
        remaining--;
    }
    update();
    countdownInterval = setInterval(update, 1000);
}
function updateExpiryTimer() {
    const expiryTime = new Date(Date.now() + REFRESH_SECONDS * 1000);
    qrExpiry.textContent = "QR hết hạn vào: " + expiryTime.toLocaleTimeString();
}
function updateAllWithExpiry() {
    updateAll();
    updateExpiryTimer();
    startCountdown();
    saveLocalStorage();
}

// ============== LocalStorage ==============
function loadLocalStorage() {
    ["bankSearch", "accountNo", "receiverName", "amount", "desc", "qrSize", "qrColor", "qrColorText"].forEach(id => {
        const val = localStorage.getItem(id);
        if (val) document.getElementById(id).value = val;
    });
    const savedBin = localStorage.getItem("bankBin");
    if (savedBin) searchInput.dataset.bin = savedBin;

    const savedAvatar = localStorage.getItem("avatar");
    if (savedAvatar) {
        avatarPreview.src = savedAvatar;
        avatarPreview.classList.remove("hidden");
        previewWrapper.classList.remove("hidden");
    } else {
        avatarPreview.classList.add("hidden");
        previewWrapper.classList.add("hidden");
    }
}
function saveLocalStorage() {
    ["bankSearch", "accountNo", "receiverName", "amount", "desc", "qrSize", "qrColor", "qrColorText"].forEach(id => {
        localStorage.setItem(id, document.getElementById(id).value);
    });
    localStorage.setItem("bankBin", searchInput.dataset.bin || "");
    if (localStorage.getItem("avatar") || avatarPreview.src) {
        localStorage.setItem("avatar", localStorage.getItem("avatar"));
    } else {
        localStorage.removeItem("avatar");
    }
}

// ============== Sự kiện input ==============
["accountNo", "receiverName", "amount", "desc", "qrSize", "qrColor"].forEach(id => {
    document.getElementById(id).addEventListener("input", updateAllWithExpiry);
});
avatarInput.addEventListener("change", () => {
    const file = avatarInput.files[0];
    if (file) {
        const reader = new FileReader();
        reader.onload = function (e) {
            const base64 = e.target.result;
            avatarPreview.src = base64;
            avatarPreview.classList.remove("hidden");
            localStorage.setItem("avatar", base64);
            updateAllWithExpiry();
        };
        reader.readAsDataURL(file);
        previewWrapper.classList.remove("hidden");
    } else {
        previewWrapper.classList.add("hidden");
        avatarPreview.classList.add("hidden");
        localStorage.removeItem("avatar");
        updateAllWithExpiry();
    }
});
searchInput.addEventListener("input", debounce(() => {
    const kw = searchInput.value.trim().toLowerCase();
    bankList.innerHTML = "";
    if (!kw) { bankList.classList.add("hidden"); return; }
    const filtered = bankArray.filter(b =>
        (b.shortName || "").toLowerCase().includes(kw) ||
        (b.name || "").toLowerCase().includes(kw) ||
        (b.bin || "").toLowerCase().includes(kw)
    ).slice(0, 50);
    if (filtered.length === 0) { bankList.classList.add("hidden"); return; }
    filtered.forEach(bank => {
        const li = document.createElement("li");
        li.className = "px-3 py-2 hover:bg-gray-100 cursor-pointer";
        li.textContent = `${bank.shortName} - ${bank.name}`;
        li.addEventListener("click", () => selectBank(bank));
        bankList.appendChild(li);
    });
    bankList.classList.remove("hidden");
}));
document.getElementById("refreshBtn")?.addEventListener("click", updateAllWithExpiry);

copyBtn?.addEventListener("click", () => {
    navigator.clipboard.writeText(previewLink.value)
        .then(() => alert("Đã copy link!"))
        .catch(() => alert("Copy thất bại!"));
});

// reset btn
document.getElementById("newBtn")?.addEventListener("click", () => {
    document.getElementById("qrForm").reset();
    avatarPreview.src = "";
    avatarPreview.classList.add("hidden");
    searchInput.value = "";
    searchInput.dataset.bin = "";
    document.getElementById("bankInfo").classList.add("hidden");
    ["bankSearch", "accountNo", "receiverName", "amount", "desc", "qrSize", "qrColor", "bankBin"]
        .forEach(k => localStorage.removeItem(k));
    qrResult.classList.add("hidden");
    emvPreview.classList.add("hidden");
    document.getElementById("downloadBtn").classList.add("hidden");
    clearInterval(countdownInterval);
    document.getElementById("countdown").innerText = "--:--";
});

// Submit form
document.getElementById("qrForm").addEventListener("submit", (e) => {
    e.preventDefault();
    updateAllWithExpiry();
});

// // ============== Khởi tạo lần đầu ============== 
document.addEventListener("DOMContentLoaded", async () => {
    await loadBanks();
    loadLocalStorage();
    updateAllWithExpiry();
});
