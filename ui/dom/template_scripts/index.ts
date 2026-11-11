import { registerScripts } from "../scripts";
import { chat_full } from "./chat_full";
import { disclosure } from "./disclosure";
import { outgoing } from "./outgoing";

registerScripts({
  "chat-full": chat_full,
  outgoing,
  disclosure,
});
