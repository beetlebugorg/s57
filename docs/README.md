# S-57 Parser Documentation

This directory contains the documentation website for the S-57 Parser library, built with [Docusaurus](https://docusaurus.io/).

## Development

### Installation

Install dependencies using bun:

```bash
bun install
```

### Local Development

Start the development server:

```bash
bun start
```

This opens a browser window at `http://localhost:3000`. Most changes are reflected live without restarting the server.

### Build

Build the static website:

```bash
bun run build
```

This generates static content in the `build` directory.

### Serve

Test the production build locally:

```bash
bun run serve
```

## Deployment

The documentation is automatically deployed to GitHub Pages when changes are pushed to the main branch.

### Manual Deployment

```bash
GIT_USER=<Your GitHub username> bun run deploy
```

## Structure

```
docs/
├── docs/                    # Documentation pages
│   ├── Introduction.md      # Landing page
│   ├── installation.md      # Installation guide
│   ├── api-reference.md     # Complete API reference
│   └── examples.md          # Code examples
├── examples/                # Runnable code examples
├── src/                     # Custom React components
├── static/                  # Static assets (images, etc.)
├── docusaurus.config.js     # Docusaurus configuration
├── sidebars.js              # Sidebar navigation
└── package.json             # Dependencies
```

## Adding Content

### New Documentation Page

1. Create a new `.md` file in `docs/`
2. Add frontmatter:
   ```markdown
   ---
   sidebar_position: 3
   ---
   # Your Title
   ```
3. Update `sidebars.js` if needed

### New Example

1. Create directory in `examples/` (e.g., `examples/01-basic-parsing/`)
2. Add Go code with explanatory `README.md`
3. Include working examples that users can copy

### Images and Assets

Place in `static/img/` and reference as:
```markdown
![Alt text](/img/filename.png)
```

## Technology

- **Docusaurus 3.7** - Static site generator
- **React 19** - UI framework
- **Bun** - Package manager and runtime
- **MDX** - Enhanced Markdown with React components
