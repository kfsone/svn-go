package svn

/*
revision -> revision-number revision-props <newline> node-list

revision-number -> "Revision-number: " <digits>:revision-number <newline>

revision-props ->
  "Prop-content-length: " <digits>:prop-content-length <newline>
  "Content-length: " <digits>:prop-content-length <newline> <newline>
  <prop-content-length bytes of property-data>

property-data -> key-value-pair-list props-end

key-value-pair-list -> key-value-pair *

key-value-pair -> key value

key -> "K " digit:key-length <newline> <key-length bytes of data> <newline>

value -> "V " digit:value-length <newline> <value-length bytes of data> <newline>

props-end -> "PROPS-END" <newline>

node-list -> node *

node -> node-header node-content <newline> <newline>

node-header ->
  Node-path: <newline-terminated-string>
  Node-kind: "file" | "dir"
  Node-action: "change" | "add" | "delete" | "replace"
  [Node-copyfrom-rev: <newline-terminated-string>]
  [Node-copyfrom-path: <newline-terminated-string>]

node-content -> node-content-header node-content-body

node-content-header ->
  [Text-copy-source-md5: <newline-terminated-string>]
  [Text-content-md5: <newline-terminated-string>]
  [Text-content-length: <digits>:node-text-content-length]
  [Prop-content-length: <digits>:node-prop-content-length]
  Content-length: <digits>:node-content-length <newline> <newline>

node-content-body ->
  <node-prop-content-length bytes of property-data>
  <node-text-content-length bytes of <string> >
*/
