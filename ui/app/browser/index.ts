import "../../assets/ui";
import { registerScripts } from "../../dom/scripts";
import { chat_rail } from "./chat_rail";
import { content_navigation } from "./content_navigation";
import { mail_navigation } from "./mail_navigation";
import { mail_sidebar } from "./mail_sidebar";

registerScripts({
  "chat-rail": chat_rail,
  "content-navigation": content_navigation,
  "mail-navigation": mail_navigation,
  "mail-sidebar": mail_sidebar,
});
