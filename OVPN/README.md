# Perfis OpenVPN

Coloque aqui dentro todos os arquivos `.ovpn` que devem ser importados pelo
módulo `ovpn` (`internal/modules/sistema.go`) durante o setup.

O módulo lê todo `*.ovpn` presente nesta pasta e importa cada um como uma
conexão do NetworkManager. Se um arquivo declarar `dhcp-option DNS` e/ou
`dhcp-option DOMAIN`, esses valores são aplicados automaticamente via `nmcli`.

Os arquivos `.ovpn` (e certificados/chaves que venham com eles) não são
versionados — só este README fica no repositório (veja `.gitignore`).
