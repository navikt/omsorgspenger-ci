# omsorgspenger-ci
Hjelpefiler for deployment og ci

#bruk
Stå i prosjekt og skriv

```
deploy dev-sbs
deploy dev-fss
```
# deploy
```bash
brew install go
mkdir -p ~/go/bin
echo "export PATH=\"$HOME/go/bin:$PATH\"" >> $HOME/.zshrc
git clone git@github.com:navikt/omsorgspenger-ci.git
cd deploy
go install
source $HOME/.zshrc
```
