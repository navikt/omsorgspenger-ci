# omsorgspenger-ci
Hjelpefiler for deployment og ci

# deploy
```bash
brew install go
mkdir -p ~/go/bin
echo "export PATH=\"$HOME/go/bin:$PATH\"" >> $HOME/.zsh
git clone git@github.com:navikt/omsorgspenger-ci.git
cd deploy
go install
source $HOME/.zsh
```
