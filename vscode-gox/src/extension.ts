import { workspace, ExtensionContext, commands, extensions } from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export async function activate(context: ExtensionContext) {
  // Get the gox executable path from settings
  const config = workspace.getConfiguration('gox');
  const goxPath = config.get<string>('lsp.path') || 'gox';

  // Check if the Go extension is installed and active
  const goExtension = extensions.getExtension('golang.go');
  const goExtensionActive = goExtension?.isActive ?? false;

  // Server options - run gox lsp
  const serverOptions: ServerOptions = {
    command: goxPath,
    args: ['lsp'],
  };

  // If Go extension is active, only handle .gox files to avoid conflicts
  // Otherwise, handle both .gox and .go files
  const documentSelector = goExtensionActive
    ? [{ scheme: 'file', language: 'gox' }]
    : [
        { scheme: 'file', language: 'gox' },
        { scheme: 'file', language: 'go' },
      ];

  const filePattern = goExtensionActive ? '**/*.gox' : '**/*.{gox,go}';

  const clientOptions: LanguageClientOptions = {
    documentSelector,
    synchronize: {
      fileEvents: workspace.createFileSystemWatcher(filePattern),
    },
  };

  // Create and start the client
  client = new LanguageClient(
    'goxLanguageServer',
    'Gox Language Server',
    serverOptions,
    clientOptions
  );

  if (goExtensionActive) {
    console.log('Gox LSP activated for .gox files only (Go extension detected)');
  } else {
    console.log('Gox LSP activated for .gox and .go files');
  }

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
