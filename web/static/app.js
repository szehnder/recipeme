"use strict";

const state = {
  prompt: "",
  searchTerms: [],
  page: 1,
  recipes: [],
  buffer: [],
  saved: new Set(),       // IDs of thumbs-up'd recipes
  savedRecipes: [],       // full recipe objects for POST /api/save
  seenIds: new Set(),     // all fetched IDs (sent with /more for dedup)
  loading: false,
};

// ── Helpers ──────────────────────────────────────────────────────────────────

function updateBadge() {
  const btn = document.getElementById("save-btn");
  const badge = document.getElementById("save-badge");
  const count = state.saved.size;
  badge.textContent = count;
  if (count === 0) {
    btn.classList.add("hidden");
  } else {
    btn.classList.remove("hidden");
  }
}

function updateHeaderTerms() {
  const queryEl = document.getElementById("header-query");
  const termsEl = document.getElementById("header-terms");

  if (state.prompt) {
    queryEl.textContent = `"${state.prompt}"`;
  }

  if (state.searchTerms && state.searchTerms.length > 0) {
    termsEl.textContent = "Searching: " + state.searchTerms.join(" · ");
  }
}

// ── Tile Building ─────────────────────────────────────────────────────────────

function buildTile(recipe) {
  const tile = document.createElement("div");
  tile.className = "tile";
  tile.dataset.id = recipe.id;

  const img = document.createElement("img");
  img.src = recipe.image || "";
  img.alt = recipe.title || "";
  img.loading = "lazy";
  tile.appendChild(img);

  const overlay = document.createElement("div");
  overlay.className = "overlay";

  const titleEl = document.createElement("div");
  titleEl.className = "recipe-title";
  titleEl.textContent = recipe.title || "Untitled";
  overlay.appendChild(titleEl);

  const actions = document.createElement("div");
  actions.className = "actions";

  const btnUp = document.createElement("button");
  btnUp.className = "btn-up";
  btnUp.title = "Save this recipe";
  btnUp.textContent = "\u25B2";
  btnUp.addEventListener("click", (e) => {
    e.stopPropagation();
    if (!state.saved.has(recipe.id)) {
      state.saved.add(recipe.id);
      state.savedRecipes.push(recipe);
      tile.classList.add("voted-up");
      updateBadge();
    }
  });

  const btnDown = document.createElement("button");
  btnDown.className = "btn-down";
  btnDown.title = "Dismiss this recipe";
  btnDown.textContent = "\u25BC";
  btnDown.addEventListener("click", (e) => {
    e.stopPropagation();
    dismissTile(recipe.id);
  });

  actions.appendChild(btnUp);
  actions.appendChild(btnDown);
  overlay.appendChild(actions);
  tile.appendChild(overlay);

  return tile;
}

// ── Dismiss ──────────────────────────────────────────────────────────────────

function dismissTile(recipeId) {
  const tile = document.querySelector(`[data-id="${recipeId}"]`);
  if (!tile) return;

  // Capture current height before CSS collapses it
  const currentHeight = tile.offsetHeight;
  tile.style.height = currentHeight + "px";

  tile.classList.add("dismissing");

  const next = state.buffer.shift();

  tile.addEventListener("transitionend", () => {
    if (next) {
      tile.replaceWith(buildTile(next));
    } else {
      tile.remove();
    }
    state.seenIds.add(recipeId);
  }, { once: true });

  if (state.buffer.length < 5 && !state.loading) {
    fetchMore();
  }
}

// ── Fetch More (for buffer) ───────────────────────────────────────────────────

async function fetchMore() {
  if (state.loading) return;
  state.loading = true;
  state.page++;

  try {
    const resp = await fetch("/api/recipes/more", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        searchTerms: state.searchTerms,
        page: state.page,
        seen: [...state.seenIds],
      }),
    });

    if (!resp.ok) {
      console.error("fetchMore failed:", resp.status);
      state.loading = false;
      return;
    }

    const data = await resp.json();
    if (data.recipes && data.recipes.length > 0) {
      data.recipes.forEach(r => {
        state.seenIds.add(r.id);
        state.buffer.push(r);
      });
    }
  } catch (err) {
    console.error("fetchMore error:", err);
  } finally {
    state.loading = false;
  }
}

// ── Load More (appends directly to grid) ──────────────────────────────────────

let loadMoreDebounceTimer = null;

async function loadMore() {
  if (loadMoreDebounceTimer) return;

  const btn = document.getElementById("load-more-btn");
  btn.disabled = true;

  loadMoreDebounceTimer = setTimeout(() => {
    loadMoreDebounceTimer = null;
    btn.disabled = false;
  }, 800);

  const prevPage = state.page;
  state.page++;

  try {
    const resp = await fetch("/api/recipes/more", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        searchTerms: state.searchTerms,
        page: state.page,
        seen: [...state.seenIds],
      }),
    });

    if (!resp.ok) {
      console.error("loadMore failed:", resp.status);
      state.page = prevPage;
      return;
    }

    const data = await resp.json();
    const grid = document.getElementById("grid");

    if (data.recipes && data.recipes.length > 0) {
      data.recipes.forEach(r => {
        state.seenIds.add(r.id);
        grid.appendChild(buildTile(r));
      });
    }
  } catch (err) {
    console.error("loadMore error:", err);
    state.page = prevPage;
  }
}

// ── Save ─────────────────────────────────────────────────────────────────────

async function saveRecipes() {
  if (state.saved.size === 0) return;

  try {
    const resp = await fetch("/api/save", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        prompt: state.prompt,
        searchTerms: state.searchTerms,
        saved: state.savedRecipes,
      }),
    });

    if (!resp.ok) {
      console.error("save failed:", resp.status);
      return;
    }

    const data = await resp.json();
    showSuccessOverlay(data.filePath, state.savedRecipes);
  } catch (err) {
    console.error("save error:", err);
  }
}

function showSuccessOverlay(filePath, recipes) {
  const overlay = document.getElementById("success-overlay");
  const pathEl = document.getElementById("success-path");
  const listEl = document.getElementById("success-list");

  pathEl.textContent = filePath || "";

  listEl.innerHTML = "";
  recipes.forEach(r => {
    const li = document.createElement("li");
    li.textContent = r.title || "Untitled";
    listEl.appendChild(li);
  });

  overlay.classList.remove("hidden");
}

// ── Initial Load ──────────────────────────────────────────────────────────────

async function init() {
  const params = new URLSearchParams(window.location.search);
  const q = params.get("q") || "";

  if (!q) {
    const grid = document.getElementById("grid");
    grid.innerHTML = '<p style="padding:40px;color:#888;font-size:13px;letter-spacing:0.5px;">No search query provided.</p>';
    return;
  }

  state.prompt = q;

  try {
    const resp = await fetch(`/api/recipes?q=${encodeURIComponent(q)}`);
    if (!resp.ok) {
      console.error("init fetch failed:", resp.status);
      return;
    }

    const data = await resp.json();

    state.prompt = data.prompt || q;
    state.searchTerms = data.searchTerms || [];
    state.recipes = data.recipes || [];
    state.buffer = data.buffer || [];

    // Track all fetched IDs
    state.recipes.forEach(r => state.seenIds.add(r.id));
    state.buffer.forEach(r => state.seenIds.add(r.id));

    updateHeaderTerms();

    const grid = document.getElementById("grid");
    grid.innerHTML = "";

    if (state.recipes.length === 0) {
      grid.innerHTML = '<p style="padding:40px;color:#888;font-size:13px;letter-spacing:0.5px;">No recipes found. Try a different search.</p>';
      return;
    }

    state.recipes.forEach(r => {
      grid.appendChild(buildTile(r));
    });

  } catch (err) {
    console.error("init error:", err);
  }
}

document.addEventListener("DOMContentLoaded", init);
