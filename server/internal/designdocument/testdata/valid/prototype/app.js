"use strict";

const KEYWORD_STORAGE_KEY = "multica.design-document.orders.keyword";

const orders = [
  { id: "CRM-2048", customer: "Lin", status: "Pending review" },
  { id: "CRM-2049", customer: "Zhao", status: "Approved" },
  { id: "CRM-2050", customer: "Chen", status: "Shipped" }
];

let descending = false;

function readSavedKeyword() {
  try {
    return window.localStorage.getItem(KEYWORD_STORAGE_KEY) || "";
  } catch (error) {
    return "";
  }
}

function saveKeyword(keyword) {
  try {
    window.localStorage.setItem(KEYWORD_STORAGE_KEY, keyword);
  } catch (error) {
    // Local storage is optional for this prototype.
  }
}

function matchesKeyword(order, keyword) {
  if (keyword === "") {
    return true;
  }
  const needle = keyword.toLowerCase();
  return order.id.toLowerCase().includes(needle) || order.customer.toLowerCase().includes(needle);
}

function renderRows(rows) {
  const body = document.getElementById("orders-body");
  body.replaceChildren();
  for (const order of rows) {
    const row = document.createElement("tr");
    for (const cell of [order.id, order.customer, order.status]) {
      const column = document.createElement("td");
      column.textContent = cell;
      row.append(column);
    }
    body.append(row);
  }
}

function render(keyword) {
  const rows = orders.filter((order) => matchesKeyword(order, keyword));
  rows.sort((left, right) => (descending ? right.id.localeCompare(left.id) : left.id.localeCompare(right.id)));
  renderRows(rows);
  document.getElementById("orders-loading").hidden = true;
  document.getElementById("orders-empty").hidden = rows.length !== 0;
  document.getElementById("orders-table").hidden = rows.length === 0;
}

document.getElementById("order-filter").addEventListener("submit", (event) => {
  event.preventDefault();
  const keyword = document.getElementById("keyword").value.trim();
  const invalid = keyword.length === 1;
  document.getElementById("keyword-error").hidden = !invalid;
  if (invalid) {
    return;
  }
  saveKeyword(keyword);
  render(keyword);
});

document.getElementById("sort-order").addEventListener("click", () => {
  descending = !descending;
  render(readSavedKeyword());
});

document.getElementById("open-filters").addEventListener("click", () => {
  document.getElementById("filter-drawer").hidden = false;
});

document.getElementById("close-filters").addEventListener("click", () => {
  document.getElementById("filter-drawer").hidden = true;
});

const savedKeyword = readSavedKeyword();
document.getElementById("keyword").value = savedKeyword;
render(savedKeyword);
