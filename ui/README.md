# Fabric-X Block Explorer UI

A modern, responsive web interface for the Fabric-X Block Explorer built with Next.js, React, and TypeScript.

## Features

- 📊 Real-time dashboard with block statistics
- 🔍 Block and transaction search functionality
- 📝 Detailed block and transaction viewers
- 🔐 Namespace policy explorer
- 📱 Responsive design for all devices
- 🎨 Modern UI with Tailwind CSS

## Prerequisites

- Node.js 18+ 
- npm or yarn
- Fabric-X Block Explorer Backend running on http://localhost:8080

## Getting Started

1. **Install dependencies:**

```bash
npm install
```

2. **Configure environment:**

```bash
cp .env.example .env
```

Edit `.env` and set the API base URL if different from default.

3. **Run development server:**

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

## Build for Production

```bash
npm run build
npm start
```

## Project Structure

```
├── app/                    # Next.js app directory
│   ├── layout.tsx         # Root layout
│   ├── page.tsx           # Dashboard page
│   ├── blocks/            # Blocks pages
│   ├── transactions/      # Transaction pages
│   └── policies/          # Policy pages
├── components/            # React components
│   ├── layout/           # Layout components
│   ├── blocks/           # Block-related components
│   ├── transactions/     # Transaction components
│   └── ui/               # Reusable UI components
├── lib/                  # Utilities and API client
└── public/               # Static assets
```

## API Configuration

The UI connects to the Fabric-X Block Explorer backend. Configure the API URL in `.env`:

```
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

## License

Apache-2.0
