// Shared domain types for hquizlet web app
// Dev 3 - core types, aligned with OpenAPI contract (Dev 5)

export type HealthStatus = "checking" | "live" | "offline";
export type AuthMode = "login" | "register";
export type AppView = "dashboard" | "editor" | "study" | "folders" | "live" | "classes" | "class-detail" | "class-create" | "class-edit" | "class-join" | "activity" | "wallet" | "deposit" | "admin-payments";

export type User = {
  id: number;
  name: string;
  email: string;
  image?: string;
  role: string;
};

export type AuthResponse = {
  authenticated: boolean;
  token: string;
  user: User;
};

export type ServiceHealth = {
  name: string;
  url: string;
  status: string;
};

export type StudySet = {
  id: number;
  title: string;
  description: string;
  thumbnailUrl?: string | null;
  flashcards?: Flashcard[];
  flashcardCount?: number;
};

export type Flashcard = {
  id: number;
  studySetId: number;
  term: string;
  definition: string;
  imageUrl?: string | null;
  starred: boolean;
};

export type DraftCard = {
  key: string;
  id?: number;
  term: string;
  definition: string;
  starred?: boolean;
};

// --- Phase 7: Class & Activity Types ---

export type ClassSummary = {
  id: number;
  name: string;
  description: string;
  inviteCode: string;
  memberCount: number;
  studySetCount: number;
  myRole: "owner" | "teacher" | "student";
  createdAt: string;
  updatedAt: string;
};

export type ClassDetail = ClassSummary & {
  maxMembers: number;
};

export type ClassMember = {
  id: number;
  classId: number;
  userId: number;
  role: "owner" | "teacher" | "student";
  joinedAt: string;
};

export type ClassStudySet = {
  classId: number;
  studySetId: number;
  title?: string;
  flashcardCount?: number;
  addedByUserId: number;
  addedAt: string;
};

export type JoinClassResponse = {
  classId: number;
  className: string;
  myRole: string;
  joinedAt: string;
};

export type ActivityItem = {
  id: number;
  eventType: string;
  entityType: string;
  entityId?: number;
  classId?: number;
  metadata?: Record<string, unknown>;
  occurredAt: string;
};

export type ActivityFeedResponse = {
  items: ActivityItem[];
  nextCursor?: string;
  hasMore: boolean;
};

// --- Phase 8: Payment, Wallet & Entitlement Types ---

export type WalletBalance = {
  balance: number;
};

export type WalletTransactionItem = {
  id: number;
  type: string;
  direction: string;
  amountVnd: number;
  label: string;
  note?: string;
  createdAt: string;
};

export type WalletTransactionList = {
  items: WalletTransactionItem[];
  total: number;
};

export type PaymentOrder = {
  orderId: number;
  orderCode: string;
  bankAccountNumber: string;
  bankAccountHolder: string;
  bankName: string;
  amountVnd: number;
  qrCodeUrl: string;
};

export type DepositOrderStatus = {
  orderId: number;
  status: string;
  amountVnd: number;
  createdAt: string;
  qrCodeUrl: string;
};

export type StudySetAccessInfo = {
  pricingType: string;
  priceVnd: number;
  hasAccess: boolean;
  requiresPurchase: boolean;
  isOwner: boolean;
  grantedVia?: string;
};

export type PurchaseResult = {
  balance: number;
  priceVnd: number;
};

export type AdminOrderItem = {
  id: number;
  userId: number;
  sepayOrderCode: string;
  amountVnd: number;
  status: string;
  createdAt: string;
};

export type AdminOrderList = {
  items: AdminOrderItem[];
  total: number;
};

export type AdminTxList = {
  items: WalletTransactionItem[];
  total: number;
};
