# omsorgspenger-ci
Hjelpefiler for deployment og ci

# deploy
```bash
brew install go
mkdir -p ~/go/bin
echo "export PATH=\"$HOME/go/bin:$PATH\"" >> $HOME/.zsh
source $HOME/.zsh
go get github.com/navikt/omsorgspenger-ci/deploy
```
