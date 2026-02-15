// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useState, useEffect } from "react";
import {
    Box,
    Button,
    Chip,
    IconButton,
    Paper,
    Skeleton,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Tooltip,
    Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { getUserStatusColor } from "@utils/projectStats";
import type { MockProjectUser } from "@models/mockData";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import useGetProjectUsers from "@api/useGetProjectUsers";
import AddProjectUserDialog from "@components/project-details/users/AddProjectUserDialog";

interface ProjectUsersTabProps {
    projectId: string;
}

/**
 * ProjectUsersTab component displays the list of project users in a table
 * and allows adding new users via a multi step dialog.
 *
 * @param {ProjectUsersTabProps} props - The props for the component.
 * @returns {JSX.Element} The ProjectUsersTab component.
 */
export default function ProjectUsersTab({
    projectId,
}: ProjectUsersTabProps): JSX.Element {
    const [isDialogOpen, setIsDialogOpen] = useState<boolean>(false);
    const [localUsers, setLocalUsers] = useState<MockProjectUser[]>([]);
    const [initialized, setInitialized] = useState<boolean>(false);

    const {
        data: fetchedUsers,
        isFetching,
        error,
    } = useGetProjectUsers(projectId);

    // Reset local state when projectId changes
    useEffect(() => {
        setLocalUsers([]);
        setInitialized(false);
    }, [projectId]);

    // Sync fetched data to local state for add/delete operations
    useEffect(() => {
        if (fetchedUsers && !initialized) {
            setLocalUsers(fetchedUsers);
            setInitialized(true);
        }
    }, [fetchedUsers, initialized]);

    const users = initialized ? localUsers : (fetchedUsers ?? []);

    const handleDeleteUser = (userId: string): void => {
        // TODO: Replace with actual API call to delete user
        setLocalUsers((prevUsers) =>
            prevUsers.filter((user) => user.id !== userId),
        );
    };

    const handleAddUser = (newUser: {
        email: string;
        firstName: string;
        lastName: string;
    }): void => {
        // TODO: Replace with API call to add user
        const user: MockProjectUser = {
            id: Date.now().toString(),
            firstName: newUser.firstName || "--",
            lastName: newUser.lastName || "--",
            email: newUser.email,
            status: "Invited",
        };
        setLocalUsers((prevUsers) => [...prevUsers, user]);
    };

    const renderTableSkeleton = (): JSX.Element => (
        <>
            {[1, 2, 3].map((row) => (
                <TableRow key={row}>
                    {[1, 2, 3, 4, 5].map((col) => (
                        <TableCell key={col}>
                            <Skeleton variant="text" width="80%" />
                        </TableCell>
                    ))}
                </TableRow>
            ))}
        </>
    );

    return (
        <Box>
            <Box
                sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    mb: 2,
                }}
            >
                <Typography variant="h6">Project Users</Typography>
                <Button
                    variant="contained"
                    color="primary"
                    startIcon={<Plus size={16} />}
                    onClick={() => setIsDialogOpen(true)}
                >
                    Add Project User
                </Button>
            </Box>

            <TableContainer component={Paper} variant="outlined">
                <Table>
                    <TableHead>
                        <TableRow>
                            <TableCell>First Name</TableCell>
                            <TableCell>Last Name</TableCell>
                            <TableCell>Email</TableCell>
                            <TableCell>Status</TableCell>
                            <TableCell align="right">Action</TableCell>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {isFetching ? (
                            renderTableSkeleton()
                        ) : error ? (
                            <TableRow>
                                <TableCell colSpan={5} align="center">
                                    <ErrorIndicator entityName="project users" />
                                </TableCell>
                            </TableRow>
                        ) : users.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} align="center">
                                    <Typography
                                        variant="body2"
                                        color="text.secondary"
                                        sx={{ py: 2 }}
                                    >
                                        No users found for this project.
                                    </Typography>
                                </TableCell>
                            </TableRow>
                        ) : (
                            users.map((user) => (
                                <TableRow key={user.id}>
                                    <TableCell>{user.firstName}</TableCell>
                                    <TableCell>{user.lastName}</TableCell>
                                    <TableCell>{user.email}</TableCell>
                                    <TableCell>
                                        <Chip
                                            label={user.status}
                                            size="small"
                                            variant="outlined"
                                            color={getUserStatusColor(user.status)}
                                            sx={{ font: "caption" }}
                                        />
                                    </TableCell>
                                    <TableCell align="right">
                                        <Tooltip title="Remove user">
                                            <IconButton
                                                color="error"
                                                size="small"
                                                onClick={() => handleDeleteUser(user.id)}
                                            >
                                                <Trash size={18} />
                                            </IconButton>
                                        </Tooltip>
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>
            </TableContainer>

            {/* Add Project User Dialog */}
            <AddProjectUserDialog
                open={isDialogOpen}
                onClose={() => setIsDialogOpen(false)}
                onSubmit={handleAddUser}
            />
        </Box>
    );
}
