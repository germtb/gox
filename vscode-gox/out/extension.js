"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode_1 = require("vscode");
const node_1 = require("vscode-languageclient/node");
let client;
async function activate(context) {
    // Check if any .gox files exist in the workspace
    const goxFiles = await vscode_1.workspace.findFiles('**/*.gox', '**/node_modules/**', 1);
    const hasGoxFiles = goxFiles.length > 0;
    // Get the gox executable path from settings
    const config = vscode_1.workspace.getConfiguration('gox');
    const goxPath = config.get('lsp.path') || 'gox';
    // Server options - run gox lsp
    const serverOptions = {
        command: goxPath,
        args: ['lsp'],
    };
    // Document selector - if gox files exist, handle both .go and .gox
    // This enables seamless navigation between Go and Gox files
    const documentSelector = hasGoxFiles
        ? [
            { scheme: 'file', language: 'gox' },
            { scheme: 'file', language: 'go' },
        ]
        : [{ scheme: 'file', language: 'gox' }];
    // Client options
    const clientOptions = {
        documentSelector,
        synchronize: {
            fileEvents: hasGoxFiles
                ? vscode_1.workspace.createFileSystemWatcher('**/*.{gox,go}')
                : vscode_1.workspace.createFileSystemWatcher('**/*.gox'),
        },
    };
    // Create and start the client
    client = new node_1.LanguageClient('goxLanguageServer', 'Gox Language Server', serverOptions, clientOptions);
    // Log activation mode
    if (hasGoxFiles) {
        console.log('Gox LSP activated for both .go and .gox files');
    }
    else {
        console.log('Gox LSP activated for .gox files only');
    }
    client.start();
}
function deactivate() {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
