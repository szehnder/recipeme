# RecipeMe

> Describe what you feel like eating. Get recipes instantly.

RecipeMe is a command-line tool that turns natural language descriptions into recipe ideas. Tell it what you're in the mood for, and it searches [Spoonacular](https://spoonacular.com) using AI-generated search terms — then opens a browser UI where you can browse, select, and save recipes as markdown files.

![Recipe grid showing results for "something my picky kid would love not spicy, maybe pasta"](screenshots/Screenshot_20260328_115539.png)

*Searching: mac and cheese · butter noodles · spaghetti bolognese · chicken alfredo · pasta with marinara sauce*

![Selecting a recipe from the grid](screenshots/Screenshot_20260328_115622.png)

---

## How It Works

1. **You describe a meal** — in plain language, as specific or vague as you like
2. **AI extracts search terms** — Claude or Gemini translates your description into 3–5 recipe search queries
3. **Recipes appear instantly** — Spoonacular fetches matching recipes and displays them in a browser grid
4. **Save what you like** — select recipes and press **Save** to write them as markdown files to your chosen folder

---

## Installation

### Download a pre-built binary (recommended)

Go to the [Releases page](https://github.com/szehnder/recipeme/releases) and download the binary for your platform:

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `recipeme-macos-arm64` |
| macOS (Intel) | `recipeme-macos-amd64` |
| Linux (x86_64) | `recipeme-linux-amd64` |
| Linux (ARM64) | `recipeme-linux-arm64` |
| Windows | `recipeme-windows-amd64.exe` |

On macOS/Linux, make it executable after downloading:

```bash
chmod +x recipeme-macos-arm64
mv recipeme-macos-arm64 /usr/local/bin/recipeme
```

> **macOS users:** macOS may block the binary because it is not signed by Apple. If you see a "cannot be opened" warning, run:
> ```bash
> xattr -d com.apple.quarantine /usr/local/bin/recipeme
> ```

### Build from source

Requires [Go 1.24+](https://go.dev/dl/).

```bash
go install github.com/szehnder/recipeme@latest
```

The binary is installed to `~/go/bin/`. Make sure that directory is in your `$PATH`:

```bash
export PATH="$PATH:$HOME/go/bin"
```

---

## Configuration

RecipeMe requires three things: an LLM API key, a Spoonacular API key, and (optionally) an output folder.

### Step 1 — Choose an LLM provider (pick one)

**Option A: Anthropic Claude** (recommended)

1. Get an API key at [console.anthropic.com](https://console.anthropic.com)
2. Set the environment variable:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**Option B: Google Gemini** (free tier available)

1. Get an API key at [aistudio.google.com](https://aistudio.google.com)
2. Set the environment variable:

```bash
export GEMINI_API_KEY=AIza...
```

> **Auto-detection:** If both keys are set, Anthropic is used by default. To force Gemini: `export RECIPEME_LLM_PROVIDER=gemini`

---

### Step 2 — Get a Spoonacular API key

1. Create a free account at [spoonacular.com/food-api](https://spoonacular.com/food-api)
2. The free tier includes 150 requests/day — enough for casual use
3. Set the environment variable:

```bash
export SPOONACULAR_API_KEY=your-key-here
```

---

### Step 3 — Set an output folder (optional)

Saved recipes are written as markdown files. By default they go to `~/recipeme/`.

**Obsidian users:** Point this at your vault folder and recipes appear as notes.

```bash
# Via environment variable
export RECIPEME_VAULT_PATH=/path/to/your/notes

# Or via flag at runtime
recipeme --vault /path/to/your/notes "what I want to cook"
```

---

## Usage

```bash
recipeme "something my picky kid would love, not spicy, maybe pasta"
```

RecipeMe opens a browser window with matching recipes. Use the arrow buttons on each card to rank your favorites, then click **Save** in the top-right corner to write the selected recipes to markdown and exit.

### Tips

- Be descriptive: `"quick weeknight dinner, vegetarian, Mediterranean-ish"` works better than `"vegetarian"`
- The UI loads more results automatically — scroll down and click **Load more**
- Use `--no-browser` if you want to open the URL yourself: `recipeme --no-browser "tacos"`

---

## Environment Variables & Flags

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `ANTHROPIC_API_KEY` | — | — | Anthropic API key (Option A LLM) |
| `GEMINI_API_KEY` | — | — | Google Gemini API key (Option B LLM) |
| `RECIPEME_LLM_PROVIDER` | — | auto | Force provider: `anthropic` or `gemini` |
| `SPOONACULAR_API_KEY` | — | — | Spoonacular API key (required) |
| `RECIPEME_VAULT_PATH` | `--vault` | `~/recipeme` | Folder where markdown files are saved |
| `RECIPEME_PORT` | `--port` | random | Port for the local web server |
| `RECIPEME_NO_BROWSER` | `--no-browser` | — | Set to `1` to disable auto-open (`RECIPEME_NO_BROWSER=1`) |

---

## License

[Apache 2.0](LICENSE)
