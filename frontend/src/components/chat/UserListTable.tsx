import React from "react";
import { cn } from "@/lib/utils";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Users, User, Shield } from "lucide-react";

// Definiujemy interfejs dla użytkownika czatu
export interface ChatUser {
  id: string;
  nickname: string;
  isLocal: boolean;
}

interface UserListTableProps {
  users: ChatUser[];
  className?: string;
}

export function UserListTable({ users, className }: UserListTableProps) {
  return (
    <Card className={cn("w-full", className)}>
      <CardHeader className="py-3">
        <CardTitle className="text-lg flex justify-between items-center">
          <span className="flex items-center">
            <Users className="h-4 w-4 text-blue-400 mr-2" />
            Użytkownicy ({users.length})
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="px-3 py-2">
        <div className="overflow-y-auto max-h-48">
          <table className="w-full">
            <thead className="bg-gray-900 text-xs text-gray-400 sticky top-0">
              <tr>
                <th className="p-2 text-left">Nazwa</th>
                <th className="p-2 text-left">ID</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 text-sm">
              {users.map((user) => (
                <tr
                  key={user.id}
                  className={cn(
                    "hover:bg-gray-800/50 transition-colors",
                    user.isLocal && "text-blue-400"
                  )}
                >
                  <td className="p-2 font-medium flex items-center">
                    {user.isLocal ? (
                      <Shield className="h-3.5 w-3.5 mr-1.5 text-blue-400" />
                    ) : (
                      <User className="h-3.5 w-3.5 mr-1.5 text-gray-500" />
                    )}
                    {user.nickname} {user.isLocal && <span className="text-blue-400 text-xs ml-1">(Ty)</span>}
                  </td>
                  <td className="p-2 font-mono text-xs text-gray-400">
                    {user.id.substring(0, 8)}...
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {users.length === 0 && (
          <div className="text-center py-4 text-gray-500 flex flex-col items-center">
            <Users className="h-8 w-8 mb-2 text-gray-700" />
            <span>Brak połączonych użytkowników</span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
