-- +goose Up
-- The Azure logo of the seeded azurerm provider is the Azure mark (two shapes) instead of the first
-- placeholder. A logo an admin already changed is left alone: only the seeded value is replaced.
UPDATE providers SET logo = 'data:image/svg+xml;base64,' || replace(encode(convert_to(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="300 0 1400 1125"><path fill="#0078d4" d="M1063 11 656 364 309 987h313z"/><path fill="#0078d4" d="M1117 94 944 583l333 418-646 111h1059z"/></svg>',
    'UTF8'), 'base64'), chr(10), '')
WHERE id = 'azurerm' AND logo = 'data:image/svg+xml;base64,' || replace(encode(convert_to(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24" rx="5" fill="#0078d4"/><path fill="#fff" d="M5 18 11 5h3l-3.4 7.4L16 12l-3 6z"/></svg>',
    'UTF8'), 'base64'), chr(10), '');

-- +goose Down
UPDATE providers SET logo = 'data:image/svg+xml;base64,' || replace(encode(convert_to(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect width="24" height="24" rx="5" fill="#0078d4"/><path fill="#fff" d="M5 18 11 5h3l-3.4 7.4L16 12l-3 6z"/></svg>',
    'UTF8'), 'base64'), chr(10), '')
WHERE id = 'azurerm' AND logo = 'data:image/svg+xml;base64,' || replace(encode(convert_to(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="300 0 1400 1125"><path fill="#0078d4" d="M1063 11 656 364 309 987h313z"/><path fill="#0078d4" d="M1117 94 944 583l333 418-646 111h1059z"/></svg>',
    'UTF8'), 'base64'), chr(10), '');
