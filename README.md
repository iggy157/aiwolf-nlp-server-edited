# aiwolf-nlp-server

人狼知能コンテスト（自然言語部門） のゲームサーバです。

サンプルエージェントについては、[aiwolfdial/aiwolf-nlp-agent](https://github.com/aiwolfdial/aiwolf-nlp-agent) を参考にしてください。

> [!IMPORTANT]
> 次回大会では以下の変更があります。詳細については、[aiwolfdial.github.io/aiwolf-nlp](https://aiwolfdial.github.io/aiwolf-nlp)を参照してください。
>
> - [発言の文字数制限](./doc/logic.md#発言の文字数制限について)
> - [13人ゲーム](./doc/logic.md#13人ゲーム)
> - [プレイヤー名とプロフィール](./doc/protocol.md#info)

## ドキュメント

- [プロトコルの実装について](./doc/protocol.md)
- [ゲームロジックの実装について](./doc/logic.md)

## 実行方法

デフォルトのサーバアドレスは `ws://127.0.0.1:8080/ws` です。エージェントプログラムの接続先には、このアドレスを指定してください。\
同じチーム名のエージェント同士のみをマッチングさせる自己対戦モードは、デフォルトで有効になっています。そのため、異なるチーム名のエージェント同士をマッチングさせる場合は、設定ファイルを変更してください。

### Linux

```bash
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/aiwolf-nlp-server-linux-amd64
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/default.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/example.env
# mv example.env .env
chmod u+x ./aiwolf-nlp-server-linux-amd64
./aiwolf-nlp-server-linux-amd64
```

### Windows

```bash
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/aiwolf-nlp-server-windows-amd64.exe
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/default.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/example.env
# mv example.env .env
.\aiwolf-nlp-server-windows-amd64.exe
```

### macOS (Intel)

> [!NOTE]
> 開発元が不明なアプリケーションとしてブロックされる場合があります。\
> 下記サイトを参考に、実行許可を与えてください。  
> <https://support.apple.com/ja-jp/guide/mac-help/mh40616/mac>

```bash
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/aiwolf-nlp-server-darwin-amd64
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/default.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/example.env
# mv example.env .env
chmod u+x ./aiwolf-nlp-server-darwin-amd64
./aiwolf-nlp-server-darwin-amd64
```

### macOS (Apple Silicon)

> [!NOTE]
> 開発元が不明なアプリケーションとしてブロックされる場合があります。\
> 下記サイトを参考に、実行許可を与えてください。  
> <https://support.apple.com/ja-jp/guide/mac-help/mh40616/mac>

```bash
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/aiwolf-nlp-server-darwin-arm64
curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/default.yml
# ダイナミックプロフィールを使用する場合は、以下のコマンドを実行してください。
# curl -LJO https://github.com/aiwolfdial/aiwolf-nlp-server/releases/latest/download/example.env
# mv example.env .env
chmod u+x ./aiwolf-nlp-server-darwin-arm64
./aiwolf-nlp-server-darwin-arm64
```
