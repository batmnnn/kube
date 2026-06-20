// API calls go through nginx reverse proxy at /api — same pattern as K8s Ingress routing.
const API = "/api";

async function fetchJSON(path, options = {}) {
  const res = await fetch(`${API}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

function statusBadge(status) {
  return `<span class="status ${status}">${status}</span>`;
}

function renderOrders(orders) {
  const el = document.getElementById("orders");
  if (!orders.length) {
    el.innerHTML = '<p class="muted">No orders yet. Create one above!</p>';
    return;
  }
  el.innerHTML = `
    <div class="order-row header">
      <span>ID</span><span>Product</span><span>Qty</span><span>Status</span><span>Created</span>
    </div>
    ${orders.map(o => `
      <div class="order-row">
        <span>#${o.id}</span>
        <span>${escapeHtml(o.product)}</span>
        <span>${o.quantity}</span>
        <span>${statusBadge(o.status)}</span>
        <span>${new Date(o.created_at).toLocaleString()}</span>
      </div>
    `).join("")}
  `;
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

async function loadOrders() {
  try {
    const orders = await fetchJSON("/orders");
    renderOrders(orders);
  } catch (err) {
    document.getElementById("orders").innerHTML =
      `<p class="muted">Failed to load orders: ${escapeHtml(err.message)}</p>`;
  }
}

async function checkHealth() {
  const badge = document.getElementById("cluster-info");
  try {
    const res = await fetch("/health");
    if (res.ok) {
      badge.textContent = "API healthy";
      badge.style.color = "var(--success)";
    } else {
      badge.textContent = "API degraded";
    }
  } catch {
    badge.textContent = "API unreachable";
    badge.style.color = "var(--danger)";
  }
}

document.getElementById("order-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const msg = document.getElementById("form-message");
  msg.hidden = true;

  const product = document.getElementById("product").value.trim();
  const quantity = parseInt(document.getElementById("quantity").value, 10);

  try {
    await fetchJSON("/orders", {
      method: "POST",
      body: JSON.stringify({ product, quantity }),
    });
    msg.textContent = "Order created! Worker will process it shortly.";
    msg.hidden = false;
    document.getElementById("product").value = "";
    document.getElementById("quantity").value = "1";
    await loadOrders();
  } catch (err) {
    msg.textContent = `Error: ${err.message}`;
    msg.style.color = "var(--danger)";
    msg.hidden = false;
  }
});

document.getElementById("refresh").addEventListener("click", loadOrders);

loadOrders();
checkHealth();
setInterval(loadOrders, 5000);
