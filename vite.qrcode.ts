/** Both sites load the encoder on demand, as a chunk of its own named for what it is: third-party code. */
export const QRCODE_CHUNK = { name: 'vendor-qrcode', test: /node_modules\/qrcode-generator\// };
