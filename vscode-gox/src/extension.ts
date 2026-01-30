import * as path from 'path';
import { workspace, ExtensionContext, window } from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export async function activate(context: ExtensionContext) {
  // Check if any .gox files exist in the workspace
  const goxFiles = await workspace.findFiles('**/*.gox', '**/node_modules/**', 1);
  const hasGoxFiles = goxFiles.length > 0;

  // Get the gox executable path from settings
  const config = workspace.getConfiguration('gox');
  const goxPath = config.get<string>('lsp.path') || 'gox';

  // Server options - run gox lsp
  const serverOptions: ServerOptions = {
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
  const clientOptions: LanguageClientOptions = {
    documentSelector,
    synchronize: {
      fileEvents: hasGoxFiles
        ? workspace.createFileSystemWatcher('**/*.{gox,go}')
        : workspace.createFileSystemWatcher('**/*.gox'),
    },
  };

  // Create and start the client
  client = new LanguageClient(
    'goxLanguageServer',
    'Gox Language Server',
    serverOptions,
    clientOptions
  );

  // Log activation mode
  if (hasGoxFiles) {
    console.log('Gox LSP activated for both .go and .gox files');
  } else {
    console.log('Gox LSP activated for .gox files only');
  }

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
