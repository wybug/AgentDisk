#!/usr/bin/env node
/**
 * Mock Agent SSE Server
 * Replays recorded SSE fixture files for fast browser testing.
 *
 * Usage: node mock-agent-server.js [port] [fixture-dir]
 *   port        - HTTP port (default 9876)
 *   fixture-dir - directory containing .txt SSE fixture files
 */
const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = parseInt(process.argv[2] || '9876', 10);
const FIXTURE_DIR = process.argv[3] || __dirname;

const server = http.createServer((req, res) => {
  if (req.method === 'OPTIONS') {
    res.writeHead(200, { 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Headers': '*' });
    return res.end();
  }

  // Parse request body
  let body = '';
  req.on('data', chunk => { body += chunk; });
  req.on('end', () => {
    let prompt = '';
    try {
      const parsed = JSON.parse(body);
      const contents = parsed.input?.[0]?.content;
      if (Array.isArray(contents)) {
        prompt = contents.map(c => c.text || '').join(' ');
      }
    } catch {}

    // Choose fixture based on prompt keywords
    let fixtureFile;
    if (prompt.includes('报告') || prompt.includes('法律')) {
      fixtureFile = 'agent-sse-report.txt';
    } else {
      fixtureFile = 'agent-sse-simple.txt';
    }

    const fixturePath = path.join(FIXTURE_DIR, fixtureFile);
    if (!fs.existsSync(fixturePath)) {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      return res.end(JSON.stringify({ error: `Fixture not found: ${fixtureFile}` }));
    }

    console.log(`[MockAgent] ${req.method} ${req.url} -> ${fixtureFile} (prompt: "${prompt.substring(0, 50)}...")`);

    res.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
      'Access-Control-Allow-Origin': '*',
    });
    res.flushHeaders();

    // Read fixture and split into complete SSE events
    const content = fs.readFileSync(fixturePath, 'utf-8');
    // Split on double-newline (SSE event boundary)
    const events = content.split(/\n\n+/).filter(e => e.trim().length > 0);
    let idx = 0;

    function sendNextEvent() {
      if (idx >= events.length) {
        res.end();
        console.log(`[MockAgent] stream ended (${events.length} events)`);
        return;
      }

      const event = events[idx];
      idx++;

      // Send the complete event block with proper SSE terminator
      res.write(event.trim() + '\n\n');

      // Delay only every N events for speed
      if (idx % 3 === 0) {
        setTimeout(sendNextEvent, 5);
      } else {
        sendNextEvent();
      }
    }

    sendNextEvent();
  });
});

server.listen(PORT, () => {
  console.log(`[MockAgent] Server listening on http://localhost:${PORT}`);
  console.log(`[MockAgent] Fixture dir: ${FIXTURE_DIR}`);
});

process.on('SIGTERM', () => { server.close(); process.exit(0); });
process.on('SIGINT', () => { server.close(); process.exit(0); });
