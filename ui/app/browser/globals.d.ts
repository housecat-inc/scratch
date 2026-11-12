import "../../assets/ui";

declare global {
  interface HTMLElement {
    __hcToolCleanups?: (() => void)[];
    __hcComposer?: {
      draftId: () => string;
      focus: () => void;
      leave: () => Promise<void>;
      save: () => void;
    };
  }
  interface DocumentEventMap {
    "tour:reveal": CustomEvent<{ reveal?: string }>;
    "settings:saved": CustomEvent<{ savedMs?: number }>;
  }
  interface Window {
    openMailMoreTools?: (options?: { force?: boolean; query?: string; drill?: string }) => void;
    openPalette?: (options?: { force?: boolean; query?: string; drill?: string }) => void;
    HousecatMailPull?: { threshold: number };
    openHelpDialog?: () => void;
    hcMailSetRowReadState?: (row: Element, unread: boolean) => void;
    hcMailRowActions?: {
      archive: (row: HTMLElement) => void;
      important: (row: HTMLElement, on: boolean) => void;
      label: (row: HTMLElement, id: string, name: string, on: boolean) => void;
      labelApplied: (row: HTMLElement, id: string) => boolean;
      move: (row: HTMLElement, id: string, name: string) => void;
      readState: (row: HTMLElement, on: boolean) => void;
      star: (row: HTMLElement, on: boolean) => void;
      unarchive: (row: HTMLElement) => void;
    };
    __hcChatRailBound?: boolean;
    __hcForwardOffBound?: boolean;
    __hcTemplateListBound?: boolean;
    __mailViewPlacement?: boolean;
    hcContactNewCompanyBound?: boolean;
    hcContentNavigationBound?: boolean;
    hcProfileTimezoneBound?: boolean;
    hcReplySidebarBound?: boolean;
    hcSettingsMenuBound?: boolean;
    hcSettingsScrollerBound?: boolean;
    hcSettingsViewBound?: boolean;
    hcWorkflowActivityFilterBound?: boolean;
  }
}
