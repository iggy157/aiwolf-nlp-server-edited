# 設定ファイルについて

> [!IMPORTANT]
> 準備中のため、説明が不十分である可能性があります。

## server (サーバ設定)

### web_socket (WebSocketの設定)

- `host`: WebSocketサーバのホスト名
  同一マシン内で接続する場合は `127.0.0.1` を指定してください。
  ローカル内のマシンや外部から接続する場合は `0.0.0.0` を指定してください。
- `port`: WebSocketサーバのポート番号
  基本的に変更する必要はありません。

### authentication (認証の設定)

- `enable`: トークンによる接続認証を有効にするかどうか
  基本的には `false` で問題ありません。
- `secret`: トークンの生成に使用する秘密鍵

## game (ゲーム設定)

- `agent_count`: 1ゲームあたりのエージェント数
  5人ゲームの場合は `5`、13人ゲームの場合は `13` を指定してください。

### custom_profile (カスタムプロフィールの設定)

- `enable`: カスタムプロフィールを有効にするかどうか
  基本的には `true` で問題ありません。

#### profiles (各エージェントのカスタムプロフィール)

- `name`: エージェントの名前
- `avatar_url`: エージェントのアバター画像のURL
- `age`: エージェントの年齢
- `sex`: エージェントの性別
- `personality`: エージェントの性格

#### dynamic_profile (動的プロフィールの設定)

- `enable`: 動的プロフィールを有効にするかどうか
  デバッグ目的の場合は `false` で問題ありません。
  本戦では事前に準備したカスタムプロフィール(`custom_profile`に記述されているもの)ではなく、ChatGPTを使用して動的にプロフィールを生成します。そのため、より本戦に近い環境で動作させるためには、`true` にしてください。`.env` ファイルに `OPENAI_API_KEY` を設定する必要があります。
- `prompt`: プロフィール生成のためのプロンプト
- `attempts`: プロフィール生成の試行回数
- `avatars`: プロフィール生成に使用するアバター画像のURL

- `vote_visibility`: 投票の結果を公開するかどうか
- `talk_on_first_day`: 0日目に囁きフェーズを開始するかどうか
- `max_continue_error_ratio`: ゲームを継続するエラーエージェントの最大割合

### talk (トークフェーズの設定)

#### max_count (発言回数の設定)

- `per_agent`: 1日あたりの1エージェントの最大発言回数
- `per_day`: 1日あたりの全体の発言回数

#### max_length (発言の文字数制限の設定)

- `count_in_word`: 単語数でカウントするかどうか
- `per_talk`: 1回のトークあたりの最大文字数 制限無しの場合は-1
- `mention_length`: 1回のトークあたりのメンションを含む場合の追加文字数
- `per_agent`: 1日あたりの1エージェントの最大文字数 制限無しの場合は-1
- `base_length`: 1日あたりの1エージェントの最大文字数に含まない最低文字数 制限無しの場合は-1

- `max_skip`: 1日あたりの1エージェントの最大スキップ回数

### whisper (囁きフェーズの設定)

[talk (トークフェーズの設定)](#talk-トークフェーズの設定)と同様です。

### vote (追放フェーズの設定)

- `max_count`: 1位タイの場合の最大再投票回数

### attack (襲撃フェーズの設定)

- `max_count`: 1位タイの場合の最大再投票回数
- `allow_no_target`: 襲撃なしの日を許可するか

### timeout (タイムアウトの設定)

- `action`: エージェントのアクションのタイムアウト時間
- `response`: エージェントのヘルスチェックのタイムアウト時間`
- `acceptable`: サーバ側での猶予時間

## json_logger (JSONロガーの設定)

- `enable`: JSONログの出力を有効にするかどうか
- `output_dir`: JSONログの出力先ディレクトリ
- `filename`: JSONログのファイル名
  拡張子は不要です。`{game_id}` でゲームIDが置換されます。`{timestamp}` でタイムスタンプが置換されます。`{teams}` でチーム名が置換されます。

## game_logger (ゲームロガーの設定)

- `enable`: ゲームログの出力を有効にするかどうか
- `output_dir`: ゲームログの出力先ディレクトリ
- `filename`: ゲームログのファイル名
  拡張子は不要です。`{game_id}` でゲームIDが置換されます。`{timestamp}` でタイムスタンプが置換されます。`{teams}` でチーム名が置換されます。

> [!NOTE]
> `json_logger` はサーバと各エージェントの通信をJSON形式で記録します。\
> `game_logger` はゲームの進行を記録します。\
> `game_logger` は従来のゲームサーバ([aiwolfdial/AIWolfNLPServer](https://github.com/aiwolfdial/AIWolfNLPServer))と互換性があります。

## realtime_broadcaster (リアルタイムブロードキャスターの設定)

- `enable`: リアルタイムブロードキャスターを有効にするかどうか

> [!NOTE]
> リアルタイムブロードキャスターは、ゲームの進行をリアルタイムで配信するための機能です。\
> [aiwolfdial.github.io/aiwolf-nlp-viewer/realtime](https://aiwolfdial.github.io/aiwolf-nlp-viewer/realtime) で確認できます。

## tts_broadcaster (TTSブロードキャスターの設定)

- `enable`: TTSブロードキャスターを有効にするかどうか
  開発中の機能のため `false` で問題ありません。

> [!NOTE]
> TTSブロードキャスターは、ゲーム内の発言を音声で再生するための機能です。\
> [VOICEVOX/voicevox_engine](https://github.com/VOICEVOX/voicevox_engine)を使用することで、音声合成を行います。

## matching (マッチングの設定)

- `self_match`: 同じチーム名のエージェント同士のみをマッチングさせるかどうか
  基本的には `true` で問題ありません。
- `is_optimize`: 最適化した組み合わせマッチングを有効にするかどうか (`self_match` が `false` の場合に限る)
  基本的には `false` で問題ありません。
- `team_count`: 参加するチーム数 (`is_optimize` が `true` の場合に限る)
- `game_count`: 全体のゲーム数 (`is_optimize` が `true` の場合に限る)
- `output_path`: マッチ履歴の出力ファイル (`is_optimize` が `true` の場合に限る)
- `infinite_loop`: 組み合わせマッチングがすべて終了した場合に全体のゲーム数分のゲームを追加するかどうか (`is_optimize` が `true` の場合に限る)
  基本的には `false` で問題ありません。
